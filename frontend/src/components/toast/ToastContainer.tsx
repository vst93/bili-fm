import { useState, useEffect } from "react";

import Toast from "./Toast";

export interface ToastItem {
  id: number;
  type: "success" | "error" | "info" | "warning";
  content: string;
  duration?: number;
}

const ToastContainer = () => {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  useEffect(() => {
    const handleToast = (event: CustomEvent<ToastItem>) => {
      const newToast = {
        ...event.detail,
        id: Date.now(),
      };

      setToasts((prev) => [...prev, newToast]);
    };

    window.addEventListener("show-toast" as any, handleToast);

    return () => {
      window.removeEventListener("show-toast" as any, handleToast);
    };
  }, []);

  const removeToast = (id: number) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  };

  return (
    <div className="fixed top-0 right-0 z-50 p-4 space-y-2">
      {toasts.map((toast) => (
        <Toast
          key={toast.id}
          content={toast.content}
          duration={toast.duration}
          type={toast.type}
          onClose={() => removeToast(toast.id)}
        />
      ))}
    </div>
  );
};

export default ToastContainer;
