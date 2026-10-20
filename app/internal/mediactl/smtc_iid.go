package mediactl

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

// typedEventHandlerIID 是 WinRT 参数化 TypedEventHandler 的接口 ID（由
// Windows 运行时类型签名 SHA-1 派生）。Windows 不列出这类 IID，生成规则
// 来自 Windows Runtime 的 type system；测试用已知的按钮事件 IID 钉住。
func typedEventHandlerIID(senderClass, senderIID, argsClass, argsIID string) string {
	const typed = "9de1c534-6ae1-11e0-84e1-18a905bcc53f"
	sig := fmt.Sprintf("pinterface({%s};rc(%s;{%s});rc(%s;{%s}))", typed, senderClass, senderIID, argsClass, argsIID)
	namespace := []byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}
	h := sha1.Sum(append(namespace, sig...))
	h[6] = h[6]&0x0f | 0x50
	h[8] = h[8]&0x3f | 0x80
	return fmt.Sprintf("{%08x-%04x-%04x-%04x-%012x}",
		binary.BigEndian.Uint32(h[0:4]), binary.BigEndian.Uint16(h[4:6]),
		binary.BigEndian.Uint16(h[6:8]), binary.BigEndian.Uint16(h[8:10]), h[10:16])
}
