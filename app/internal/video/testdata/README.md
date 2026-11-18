# 测试素材

`rgb.mp4` 来自 mygo v0.9.0 的 `ui/video/testdata/rgb.mp4`（MIT License，
Copyright (c) 2026 MyGo contributors）。1.2 秒、10fps、64×48 的 H.264 + 静音
AAC：0.4 秒红、再绿、再蓝，无 B 帧。

为什么复制一份而不是从 `$GOMODCACHE` 里读：`go test` 的包目录只保证包含本模块
的文件，跨模块读 testdata 在 CI（`GOFLAGS=-mod=mod` + 干净缓存）里不成立。

它只用于无头验证「原生视频路径能起播、能 seek、能改倍速」——真实的 B 站流由
`TestNativeRealBiliURL` 在给出环境变量时验证（见 native_test.go）。

`fastplay.mp4` 是合成的 8 秒 H.264/AAC 测试片，每秒 30 帧，亮度逐帧变化，每 4 秒一个关键帧。`TestProgressiveThreeTimesPlayback` 从本地 HTTP 播放它，同时检查媒体时钟和解码出的不同画面数。
