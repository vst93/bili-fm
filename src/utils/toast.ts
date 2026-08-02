interface ToastOptions {
  type: "success" | "error" | "info" | "warning";
  content: string;
  duration?: number;
}

export const toast = ({ type, content, duration = 2000 }: ToastOptions) => {
  const event = new CustomEvent("show-toast", {
    detail: {
      type,
      content,
      duration,
    },
  });

  window.dispatchEvent(event);
};
