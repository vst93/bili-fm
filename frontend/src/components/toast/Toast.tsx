import { useEffect, useState } from "react";
import { Card } from "@heroui/react";
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

  const getIcon = () => {
    switch (type) {
      case "success":
        return <CheckOne fill="#4ade80" size="20" theme="outline" />;
      case "error":
        return <CloseOne fill="#f43f5e" size="20" theme="outline" />;
      case "warning":
        return <Attention fill="#f59e0b" size="20" theme="outline" />;
      case "info":
        return <Info fill="#0ea5e9" size="20" theme="outline" />;
    }
  };

  const getBgColor = () => {
    switch (type) {
      case "success":
        return "bg-success-50";
      case "error":
        return "bg-danger-50";
      case "warning":
        return "bg-warning-50";
      case "info":
        return "bg-primary-50";
    }
  };

  return (
    <Card
      className={`fixed top-4 right-4 z-50 transition-all duration-300 ${
        isVisible ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-2"
      } ${getBgColor()}`}
      shadow="sm"
    >
      <div className="flex items-center gap-2 px-4 py-2">
        {getIcon()}
        <span className="text-sm">{content}</span>
      </div>
    </Card>
  );
};

export default Toast;
