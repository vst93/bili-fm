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
        return <CheckOne theme="outline" size="20" fill="#4ade80" />;
      case "error":
        return <CloseOne theme="outline" size="20" fill="#f43f5e" />;
      case "warning":
        return <Attention theme="outline" size="20" fill="#f59e0b" />;
      case "info":
        return <Info theme="outline" size="20" fill="#0ea5e9" />;
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