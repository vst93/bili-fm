#!/usr/bin/env bash
# 进度条交互的端到端检查：在 Xvfb 里真跑一遍应用，用 xdotool 按下拖动再松手。
#
# 为什么需要它：进度条的「点击/拖动失效」光看代码判断不出来（改过四五次，
# 每次都是对框架 API 的理解错了，见 app/PLAN.md）。这里直接验证现象：
# 松手那一刻 `seek()` 会走 `flushProgress()`，日志里必然留下痕迹
# （未登录时是「上报播放进度失败: 未登录」，在播时是「跳转失败: ...」）。
#
# 依赖：Xvfb、xdotool、imagemagick（本地排查用，CI 不跑）。
# 用法：./check-e2e.sh [窗口宽 高]（默认 800x600，坐标按这个尺寸算）
set -u
cd "$(dirname "$0")/app" || exit 1

W=${1:-800}
H=${2:-600}
DISP=:91
LOG=$(mktemp /tmp/bilifm-e2e-XXXX.log)

command -v Xvfb >/dev/null || { echo "缺 Xvfb，跳过"; exit 0; }
command -v xdotool >/dev/null || { echo "缺 xdotool，跳过"; exit 0; }

echo "── 编译"
go build -o /tmp/bilifm-e2e . || exit 1

echo "── 起 Xvfb + 应用（$W×$H）"
Xvfb $DISP -screen 0 "${W}x${H}x24" >/dev/null 2>&1 &
XPID=$!
sleep 2
DISPLAY=$DISP /tmp/bilifm-e2e -no-tray -no-keys -search "音乐" -play 0 \
	-shot /tmp/bilifm-e2e.png -shot-delay 26s >"$LOG" 2>&1 &
APID=$!
sleep 14

# 进度条在播放栏里：左侧是 18(pad)+80(transport)+8+56(时间)+8，
# 所以它从 x≈170 开始；宽度是 1fr，右边被 56+8+34+8+40+8+34+8+40+18 占掉。
# 高度 24、垂直居中于 56 高的播放栏。这里按 800x600 的常见值取点：
#   X 170..538、Y 560..584 → 在 60% 处按下，往左拖到 20% 再松手。
X0=$((W * 448 / 800))
Y0=$((H * 572 / 600))
X1=$((W * 240 / 800))

echo "── 拖动：($X0,$Y0) → ($X1,$Y0)"
DISPLAY=$DISP xdotool mousemove "$X0" "$Y0" mousedown 1
sleep 0.4
DISPLAY=$DISP xdotool mousemove $(((X0 + X1) / 2)) "$Y0"
sleep 0.4
DISPLAY=$DISP xdotool mousemove "$X1" "$Y0"
sleep 0.4
DISPLAY=$DISP xdotool mouseup 1

wait $APID 2>/dev/null
kill $XPID 2>/dev/null

echo
if grep -qE "上报播放进度失败|跳转失败|跳转" "$LOG"; then
	echo "✓ 松手触发了 seek（进度条交互正常）"
	echo "  日志：$(grep -E '上报播放进度失败|跳转失败|跳转' "$LOG" | head -2)"
	echo "  截图：/tmp/bilifm-e2e.png（滑块应停在拖动结束的位置）"
	exit 0
fi
echo "✗ 松手没有触发 seek —— 进度条交互有问题"
echo "  完整日志：$LOG"
tail -20 "$LOG"
exit 1
