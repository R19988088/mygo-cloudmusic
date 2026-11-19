# 网易云音乐「音效」算法逆向分析

分析对象：`D:\app\music CloudMusic\` 本地安装（2022-04 版本，PC 端）
分析方式：静态分析（PE 结构、导出表、字符串表、RTTI、vtable、反汇编、预设文件格式）
日期：2026-11-19

---

## 0. 一句话结论

**能反推到"架构 + 模块 + 参数语义 + 部分默认值"这一层，反推不到"具体系数"这一层。**

| 层级 | 可反推性 | 依据 |
| --- | --- | --- |
| 用什么引擎、几个模块、串接顺序 | ✅ 完全拿到 | 导出表、RTTI、参数 dump 日志串、vtable |
| 每个模块的参数名、含义、取值范围 | ✅ 基本拿到 | 调试日志 printf 格式串 + 调用方 JSON 键名 |
| 出厂默认参数 | ✅ 部分拿到 | 构造函数里的立即数（限幅器一组已解出） |
| 各滤波器的具体系数、混响的具体拓扑 | ⚠️ 需复现/实测 | 属编译进代码的数学实现，静态只能看到常量表 |
| 每个「音效预设」的实际数值 | ❌ 静态不可得 | `.ncae` 预设体是加密的，需运行时抓取或解密 |

所以工程上的正确做法不是"抠代码"，而是：**用参数表还原设计意图 → 用标准 DSP 复现 → 用黑盒测量校准**。

---

## 1. 证据链

### 1.1 核心文件

| 文件 | 大小 | 角色 |
| --- | --- | --- |
| `neaudioeffects.dll` | 1,262,824 | **音效 DSP 全部在这里** |
| `cloudmusic.dll` | 10,821,408 | 调用方：加载预设、把参数下发给 DSP |
| `cloudmusic_util.exe` | 2,126,624 | 与音效无关，是 TagLib 标签/封面解析器 |
| `resource\3D环绕.ncae` | 113 B | 内置预设样本 |
| `Netease\CloudMusic\audioeffect\*.ncae` | 20 个 | 用户侧音效预设（下载/缓存） |

`neaudioeffects.dll` 里残留 PDB 路径：

```
D:\Sources\AudioEffects\projects\windows\build\mt\Release\neaudioeffects.pdb
```

即网易内部工程名 **AudioEffects**，Release 构建。PE 时间戳 `0x62270AB2`（2022-03-08），x86。

### 1.2 引擎归属：Wwise（Audiokinetic）

两个独立证据：

1. 导出表里有 `g_pAKPluginList` —— 这是 Wwise 静态插件注册表的固定符号名（`AK` = Audiokinetic）。
2. 字符串里有 Wwise 引擎原文告警：

```
Soundbanks have been generated with convolution reverb parameters
that do not match sound engine runtime conditions. No wet path will be heard.
```

另有 `r8brain` 重采样器的断言串（`transition bandwidth not in [0.2,50] % of nyquist`、`precision not in [15,33] bits`），说明内部含采样率转换。

结论：**这是一个 Wwise 效果机架 + 网易自研插件（EQ / FIR / Headphone / Rotate3D / StereoEnhancer / Loudness）的组合**，不是单纯的"均衡器"。

### 1.3 对外接口

导出的是 C++ 类（非 C ABI），共 3 个类 + 1 个全局表：

```
audiofx::AudioEffects        ::Create()       // 效果机架本体（对象 0x278 = 632 字节）
audiofx::IAudioEffects                        // 虚接口
NCAudioEffectsProcessor      ::Create()       // 对外的封装处理器（0x88 字节）
ReplayGainProcessor          Init/Push/GetReplayGain/Reset  // 响度分析
g_pAKPluginList                                // Wwise 插件表
```

`AudioEffects` 的 vtable（构造时写入 `0x101007c8`，即 RVA `0x1007c8`）有 **37 个槽位**，`NCAudioEffectsProcessor` 的 vtable（RVA `0xb6310`）**35 个**。即接口面很宽：每类参数都有独立的 Get/Set 和 on/off 开关。

### 1.4 调试日志暴露了完整参数表

DLL 内保留了 printf 风格的参数 dump（走 `OutputDebugString`，不落盘，所以 `debug.log` 里看不到）。原文按代码顺序：

```
Processor instance: %s
Sample Format: %d, Buffer Frames: %u, Channels: %u, Samplerate: %u, Byte per Sample: %u
BypassMode: %s
Loudness: on %s, gain %f, peak %f
Reverb: on %s
Reverb ER: enable %s, Pattern %d, RoomSize %f, SurroundDelay %f
Reverb RVB: PreDelay %f, DecayTime %f, HFDamping %f, Density %f, RoomShape %f, Quality %u, Diffusion %f, StereoWidth %f
Reverb TC: enable %s
Reverb TC Filter %d: InsertType %u, CurveType %u, Gain %f, Freq %f, Q %f
Reverb Input Level: Center %f, LFE %f
Reverb RVB Level: Front %f, Rear %f, Center %f, LFE %f
Reverb Output Level: Dry %f, ER %f, Reverb %f
Stereo Enhancer: on %s, Presence %f, Stereoizer %f, SpectrumShaper %s, Ambience %d
Rotate 3D: on %s, Velocity %f
Graph EQ: on %s, %f ×10
Bass Treble: on %s, Bass %f, Treble %f, Bass Freq %f, Bass Q %f, Treble Freq %f, Treble Q %f
Parametric EQ: on %s, Gain %f
Parametric EQ Filter %u: on %s, FilterType %u, Gain %f, Freq %f, Q %f
FIR: on %s, IR size %u
Headphone: on %s, IR size %u
Limiter: on %s, Threshold %f, Attack %f, Release %f, OutputGain %f, Ratio %f, InfiniteRatio %s
```

**关键发现**：参数表里有两个纯静态分析很难发现的模块 —— `FIR`（卷积，带 IR 长度）和 `Headphone`（耳机 HRTF / 串扰卷积）。这就是 20 个预设里那几个 8~30 KB 大文件的来源。

### 1.5 调用方的参数键名（预设 JSON 结构）

`cloudmusic.dll` 偏移 `0x93eb74` 起是一整张键名表，顺序即解析顺序：

```
audioeffect, bass, eqs, presence, treble,
setLoudnessON, setParams, setLoudnessParams,
rsize, pattern, rvb, sdelay,
sshaper, stereoizer, ambience,
diffusion, rshape, swidth,
dtime, pdelay, density, hfdamping,
band, curve, center, gain, insert, freq, dry,
velocity, lfe, rear, front, rotate, peak
```

同处还有错误分支：

```
audio_effect.cpp
Set AudioEffect  Failed. Path Empty.
SetAudioEffectParam failed. path not exist:
SetAudioEffectParam failed. read effect file error.
Switch Off AudioEffect.
```

即：**音效 = 一个"按路径读取的预设文件"，解析成上述 JSON 参数 -> 下发给 `neaudioeffects.dll`**。

### 1.6 构造函数里的默认值

`AudioEffects::Create`（RVA `0x9090`）把 0x278 字节对象清零后写入一批默认值，其中可确认的：

| 对象偏移 | 立即数 | 含义 |
| --- | --- | --- |
| +0x58 | `2` | 声道数默认 2 |
| +0x64 | `1.0f` | 输出增益 |
| +0x4C | `0x800` (2048) | 帧/缓冲规模 |
| +0x234 | `15` | Quality 枚举默认 |
| +0x25C | `0.04f` | Limiter Attack |
| +0x260 | `0.08f` | Limiter Release |
| +0x264 | `0` | Limiter Threshold |
| +0x268 | `1` | InfiniteRatio = true |
| +0x26C | `50.0f` | Limiter Ratio |

即默认限幅器是 **阈值 0、比例 50:1（近无限比）、attack 40 ms、release 80 ms、输出增益 1.0**。

---

## 2. 处理链

按参数 dump 的代码顺序（与内部串联顺序一致，仍需动态验证一步）：

```
输入 (float32, N ch)
  │
  ├─ 0. BypassMode              总旁通
  ├─ 1. Loudness                 gain / peak（响度归一）
  ├─ 2. Reverb (RoomVerb)        ER + RVB + TC 三段参数
  ├─ 3. Stereo Enhancer          presence / stereoizer / spectrumShaper / ambience
  ├─ 4. Rotate 3D                velocity（环绕旋转）
  ├─ 5. Graph EQ                 10 段定点频带增益
  ├─ 6. Bass / Treble            shelf，带独立 freq + Q
  ├─ 7. Parametric EQ            总增益 + N 个 band(type/gain/freq/Q)
  ├─ 8. FIR                      IR 卷积（长 IR 混响/染色）
  ├─ 9. Headphone                IR 卷积（耳机定向）
  └─ 10. Limiter                 threshold/attack/release/outGain/ratio/infinite
输出
```

`ReplayGainProcessor` 是旁路分析器（离线扫全曲算增益），对应 `Loudness` 那一路的数值来源。

---

## 3. 预设文件 `.ncae` 格式

20 个样本实测，容器头结构一致：

```
offset 0   "NCAE"            4B   magic
offset 4   u32               payload 长度（与文件长度差 10~38B）
offset 8   u32   0           保留
offset 12  u16   1           格式版本
offset 14  u16   1 | 2       kind：1=纯参数，2=含 IR 数据
offset 16  ...               加密载荷
```

样本分布（kind=2 的全部是大文件，与 `IR size` 对应）：

| 预设 | 字节 | kind | 熵 |
| --- | --- | --- | --- |
| HiFi现场 | 30,892 | 2 | 7.99 |
| 录音棚立体声 | 13,637 | 2 | 7.99 |
| HiFi电子管 | 8,054 | 2 | 7.97 |
| 复古收音机 | 7,904 | 2 | 7.97 |
| 怀旧卡带机 | 6,527 | 2 | 7.97 |
| 震撼全景 | 3,545 | 2 | 7.95 |
| 超重低音 | 1,774 | 2 | 7.89 |
| 高解析人声 | 822 | 2 | 7.75 |
| 清澈人声 | 828 | 1 | 7.77 |
| 音乐厅 | 106 | 1 | 6.18 |
| 环绕立体声 | 118 | 1 | 6.32 |
| 独享立体声 | 109 | 1 | 5.96 |
| 民谣音效 | 124 | 1 | 6.24 |
| 360°环绕 | 91 | 1 | 5.89 |
| 3D环绕（内置） | 113 | 1 | — |

载荷熵 ≈ 7.9~8.0 → **加密或压缩后的密文**，非明文浮点数组（若为 float32 结构，小文件应能看到 0.7 / 0.5 / 100.0 这类规律值，实测全是随机分布）。

**所以预设数值无法靠静态解密拿到**（密钥在 `cloudmusic.dll` 里，需动态断点或内存 dump 才能取）。

---

## 4. 各模块的等价 DSP 实现（可直接落地）

以下是把参数语义翻译成"标准算法"的方案，用于在 MyGo 里复现听感；不是原始代码的字节级还原。

### 4.1 Graph EQ（10 段）

固定频带、只给增益。等价实现：10 个 **RBJ peaking biquad** 级联。

```
w0 = 2*pi*f0/fs,  alpha = sin(w0)/(2*Q)
A  = 10^(dBgain/40)
b0 = 1 + alpha*A
b1 = -2*cos(w0)
b2 = 1 - alpha*A
a0 = 1 + alpha/A
a1 = -2*cos(w0)
a2 = 1 - alpha/A
```

频带按 10 段等对数间隔取（≈ 31/62/125/250/500/1k/2k/4k/8k/16k），Q 取 1.0~1.4 使相邻段 -3dB 交叠。

### 4.2 Bass / Treble

参数自带 `freq` 和 `Q`，说明是**带 Q 的 shelf**，不是一阶 shelf。用 RBJ low-shelf / high-shelf：

```
low-shelf:  A = 10^(dB/40), alpha = sin(w0)/2 * sqrt((A+1/A)*(1/S-1)+2)，S 由 Q 换算
high-shelf: 同式，b 系数镜像
```

### 4.3 Parametric EQ

`FilterType` 枚举 + 每段 Gain/Freq/Q，标准 5 类：peak / low-shelf / high-shelf / low-pass / high-pass，全部 RBJ 公式。总 `Gain` 是整体补偿增益。

### 4.4 Reverb（RoomVerb）

三段参数对应经典结构，不是卷积混响：

- **ER（early reflection）**：`Pattern`(1~4 种抽头图案) + `RoomSize` + `SurroundDelay`
  → 4~8 个延迟抽头，幅度按 1/d 衰减，延迟时间 ∝ RoomSize。
- **RVB（late reverb）**：`PreDelay` + `DecayTime` + `HFDamping` + `Density` + `RoomShape` + `Quality` + `Diffusion` + `StereoWidth`
  → **FDN（feedback delay network）** 或并行 comb + 串联 allpass（Schroeder）。
  - `DecayTime` → 反馈增益 `g = exp(-1/(fs*T60/L))`
  - `HFDamping` → 反馈环里串一阶低通 `y += a*(x-y)`
  - `Diffusion` → allpass 系数
  - `Density` → 延迟线数量/密度
  - `RoomShape` / `StereoWidth` → 各延迟线的初始延迟与左右分布
- **TC（tone control）**：`InsertType`（插在 ER 前/RVB 前/输出）+ `CurveType`（shelf/peak）+ Gain/Freq/Q → 就是几个 shelf/peak biquad。
- **Level**：`Input(Center,LFE)` / `RVB(Front,Rear,Center,LFE)` / `Output(Dry,ER,Reverb)` 是 5.1 路由权重，2.0 场景下退化为 dry/wet 两档。

### 4.5 FIR / Headphone（卷积）

`IR size` 就是冲激响应长度。等价实现：**uniform partitioned overlap-save FFT 卷积**（1024/2048 段），复杂度 O(N log N)。

- FIR 路：单声道/立体声长 IR → 音色染色、空间感（"音乐厅/录音棚/HiFi现场"这类）。
- Headphone 路：耳机定向 IR（HRTF 或简单 crossfeed）→ "独享立体声/定位"这类。

这两路是"音效差别最大"的部分，也是最难靠调参模仿的，必须拿到 IR 本身（实测或从预设解密）。

### 4.6 Stereo Enhancer

- `Presence`：2~5 kHz 高频 shelf 提升（"通透感"）
- `Stereoizer`：M/S 处理，`L' = M + k*S, R' = M - k*S`，k>1 加宽
- `SpectrumShaper`：按频谱做高频细节增强（弱信号高通提升）
- `Ambience`：环境感强度（通常与 reverb send 联动）

### 4.7 Rotate 3D

`Velocity` 是旋转速度。双耳实现：以 LFO 相位 θ(t) 驱动左右增益/微延迟

```
L = x * (1 + d*cos θ),  R = x * (1 - d*cos θ)   // 或加 ±0.2ms 延迟
```

这解释了"360°环绕 / 3D环绕"两个预设只需 91~113 字节 —— 纯控制参数。

### 4.8 Limiter

参数含 `Ratio` + `InfiniteRatio`，是**带比例的峰值限幅**（非硬 clip）：

```
env = max(peak, env*release_coef)
if env > threshold:  g = (threshold/env)^(1/ratio)  // infinite 时 ratio→∞，g = threshold/env
out = in * g * outGain
```

默认值（已解出）：threshold 0、ratio 50、attack 0.04、release 0.08、outGain 1.0。

### 4.9 Loudness / ReplayGain

`ReplayGainProcessor::Init(sampleformat, rate, channels)` + `Push` + `GetReplayGain(type)`：标准 ReplayGain 1.0 —— 150 Hz 二阶高通 → 50 ms 分块 RMS → 直方图统计取 95 分位。`Loudness(gain, peak)` 即把该增益施加到链路首端，`peak` 用于防削顶预算。

---

## 5. 拿到"真实数值"的两条可行路径

静态到此为止，剩下的是运行时：

### 路径 A：抓 `OutputDebugString`（成本最低）

参数 dump 全是 `OutputDebugString`，不进日志文件。任意监听 `Global\DBWIN_BUFFER` 的程序都能收到（DebugView 的原理，不需要调试器）：

1. 起监听进程 → 2. 网易云里逐个切换音效 → 3. 拿到每个预设的全部参数浮点值。

一次抓取即可得到 20 个预设的 EQ 增益、混响时间、限幅阈值等，且**不需要碰加密**。

### 路径 B：在解密点下断

`.ncae` 读取后必然有"解密→解析→下发"三步。在 `cloudmusic.dll` 的 `audio_effect.cpp` 相关函数（`SetAudioEffectParam failed. read effect file error.` 附近）下断，可直接拿到解密后的明文结构（含 IR 数组）。

### 路径 C：黑盒测量（不碰二进制，最稳）

回环录制法，用于校准而不需要真值：

| 目标 | 激励 | 测量 |
| --- | --- | --- |
| 频率响应（EQ/Bass/Treble/TC） | 对数扫频 20 Hz~20 kHz | 反卷积得幅频曲线，反推每段 gain/Q |
| 混响 | 脉冲/ clicks | IR → T60、predelay、ER 抽头位置 |
| FIR/Headphone | 同上 | 直接得到 IR，可当卷积核用 |
| 限幅 | 阶梯/正弦扫幅 | 输入-输出曲线 → threshold、ratio、attack/release |
| Rotate3D | 单频 | 左右通道幅度包络 → LFO 速率/深度 |

---

## 6. 在 MyGo 里落地

现状：`internal/player` 解码后是 `[]float32` 立体声（`stream.Read(dst []float32)`），`fader` 只做整体增益，`normGain` 已有一个响度归一雏形，`spectrum` 里已有 FFT。**DSP 链天然可以插在解码与 `audio.Player` 之间。**

建议顺序（与上表一致，成本从低到高）：

1. **第 1 期**：Loudness + Limiter + Graph EQ / Bass-Treble / Parametric EQ。全部是 biquad + 包络，纯 Go 无依赖，单帧开销可忽略，听感差异最明显（"超重低音""清澈人声""摇滚经典"基本靠这一档）。
2. **第 2 期**：Stereo Enhancer + Rotate3D。M/S + LFO，实现简单，负责"环绕/立体声"类预设。
3. **第 3 期**：FDN Reverb。参数语义已全，做 RoomVerb 近似。
4. **第 4 期**：FIR / Headphone 卷积。需要 partitioned FFT 卷积 + 拿到 IR（走路径 B 或 C）。

工程注意：

- 处理链建议用**每帧 float32、原地处理**，biquad 状态按声道独立保存，切换音效时做 100~200 ms 参数插值避免爆音（原实现里 `on/off` 是硬开关，但参数是连续的）。
- 采样率以 `stream.SampleRate()` 为准，biquad 系数必须按 fs 重算，不要写死 44.1 kHz。
- 限幅器放在链路最后，卷积之后再限幅，否则 IR 卷积会把峰值顶起来。

---

## 7. 本次分析的确定性边界

**确定（有直接证据）**：引擎归属、模块清单、10 段链路、全部参数名与语义、调用方 JSON 键、限幅器默认值、预设容器头结构、预设为加密载荷、两个卷积模块的存在。

**推断（合理但未验证）**：dump 顺序 = 处理顺序；EQ 为 RBJ biquad 族；Reverb 为 ER + FDN 结构；Rotate3D 为 LFO 双耳调制。

**不可得**：滤波器实际系数、混响具体拓扑与延迟线长度、各预设数值、IR 数据本身。
