# Windows 图标马赛克 / 旧图标排查

> 适用: 更新到 2.0.29-preview (或更高) 后, Windows 桌面 / 任务栏图标仍然
> 模糊、马赛克或显示旧图标。

## 结论

**源码图标与打包产物均为高质量, 问题不在工程侧。**

- `icons/icon.png`: 512×512 RGBA, 矢量风格 logo (B 站电视), 约 1800 色,
  边缘锐利, 不是低质量源图。
- `icons/icon.ico`: 8 个尺寸 (16~256), 全部 32-bit RGBA, 每个尺寸与
  512 源图的 LANCZOS 高质量缩放结果逐像素一致 (已验证零差异)。
- `tauri.conf.json` `bundle.icon` 已使用 `icon.png` (512px, 上一版本曾误用
  300px 的 `icon-square.png`, 已修复)。
- 无自定义 `build.rs` / `.rc` 图标注入, WiX/NSIS 均使用 bundle 图标, 配置正确。

**马赛克/旧图标的真正来源是 Windows 图标缓存与快捷方式 (.lnk) 内置的旧图标。**
更新程序只替换了 exe, 但 Explorer 的图标缓存和已固定的任务栏快捷方式
仍然记录着旧版本 (300px) 图标, 不会随 exe 更新自动刷新, 直到重启系统或
手动清除缓存。

## 修复步骤 (按顺序执行)

### 1. 刷新 Explorer 图标缓存 (最简单, 先试这个)

```bat
ie4uinit.exe -show
```

没有生效就重启资源管理器:

```bat
taskkill /f /im explorer.exe
start explorer.exe
```

### 2. 彻底清除图标缓存

关闭 bili-FM 后, 删除缓存文件再重启 Explorer:

```bat
taskkill /f /im explorer.exe
del /A:H "%LocalAppData%\IconCache.db"
del /A:H "%LocalAppData%\Microsoft\Windows\Explorer\iconcache_*.db"
start explorer.exe
```

> 注意: `iconcache_*.db` 在 Explorer 运行时被占用, 必须先结束 Explorer。

### 3. 任务栏固定图标 (最容易被忽略)

固定到任务栏的快捷方式在
`%AppData%\Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar\`
其 `.lnk` 文件内部缓存了旧图标位图, **更新 exe 不会刷新它**。
右键图标 → 从任务栏取消固定, 再从开始菜单重新固定。

### 4. 桌面快捷方式

如果通过应用内更新器 (updater) 升级, 安装器不会重新创建桌面快捷方式,
其中的旧图标会一直保留。删除桌面快捷方式后从开始菜单重新发送到桌面,
或重新运行安装包 (NSIS/WiX 会重建快捷方式)。

### 5. 最后手段

重启 Windows。图标缓存文件全部重建, 任务栏/桌面/开始菜单一律读取新 exe
图标。

## 验证图标确实已更新

在资源管理器中右键 bili-FM.exe → 属性 → 图标 标签页, 查看内置图标是否
是新的蓝色电视 logo。若 exe 属性里已是新图标而桌面仍是旧的, 则确定是
缓存/快捷方式问题, 按上面第 2~4 步处理。

---

## 开发侧: 如何避免再次出现

- 打包图标只使用 `icons/icon.png` (512×512) 作为源, 不要使用
  `icon-square.png` (300×300)。
- 发布新版本前用 `tauri icon <512px源图>` 重新生成全套图标 (会同时重建
  icon.ico 的所有尺寸)。
- 大版本更新时考虑在 release 说明中提示用户: 若图标未变, 先取消固定再
  重新固定任务栏图标。
