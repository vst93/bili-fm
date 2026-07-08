import { useEffect, useState } from "react";
import { CheckOne, CloseOne, Attention, Info } from "@icon-park/react";

interface ToastProps {
  type: "success" | "error" | "info" | "warning";
  content: string;
  duration?: number;
  onClose: () => void;
}

const Toast = ({ type, content, duration = 2000, onClose }: ToastProps) => {
  const [isVisible, setIsVisible] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setIsVisible(false);
      setTimeout(onClose, 300); // 等待动画结束后再移除
    }, duration);

    return () => clearTimeout(timer);
  }, [duration, onClose]);

  const getAccent = () => {
    switch (type) {
      case "success":
        return { icon: <CheckOne fill="#22c55e" size="20" theme="outline" />, tint: "rgba(34, 197, 94, 0.08)" };
      case "error":
        return { icon: <CloseOne fill="#f43f5e" size="20" theme="outline" />, tint: "rgba(244, 63, 94, 0.08)" };
      case "warning":
        return { icon: <Attention fill="#f59e0b" size="20" theme="outline" />, tint: "rgba(245, 158, 11, 0.08)" };
      case "info":
        return { icon: <Info fill="#0ea5e9" size="20" theme="outline" />, tint: "rgba(14, 165, 233, 0.08)" };
    }
  };

  const accent = getAccent();

  return (
    <div
      className={`fixed top-4 right-4 z-50 toast-glass transition-all duration-300 ${
        isVisible ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-2"
      }`}
      style={{ background: `linear-gradient(135deg, rgba(255, 255, 255, 0.5), ${accent?.tint || "rgba(255,255,255,0.4)"})` }}
    >
      <div className="flex items-center gap-2 px-4 py-2">
        {accent?.icon}
        <span className="text-sm font-medium text-slate-800">{content}</span>
      </div>
    </div>
  );
};

export default Toast;
