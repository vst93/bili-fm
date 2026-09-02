import { useState, createContext, useContext, useCallback, ReactNode } from "react";
import { CheckOne, CloseOne, Attention, Info, Refresh } from "@icon-park/react";
import { open } from "@tauri-apps/plugin-shell";

type DialogType = "info" | "success" | "warning" | "error" | "question" | "loading";

interface DialogButton {
  label: string;
  value: string;
  primary?: boolean;
  /** 若提供，点击按钮时不关闭对话框，而是调用此回调（用于在同一个对话框内
      原地切换内容，例如 检查更新 → 下载进度 → 更新完成） */
  onClick?: (id: number) => void;
}

interface DialogConfig {
  title: string;
  message: ReactNode;
  type?: DialogType;
  buttons?: DialogButton[];
  onClose?: (value: string) => void;
}

interface DialogState extends DialogConfig {
  id: number;
  visible: boolean;
}

interface DialogContextValue {
  showDialog: (config: DialogConfig) => number;
  closeDialog: (id: number, value?: string) => void;
  /** 原地更新已有对话框的内容（不关闭/重开，避免视觉跳动） */
  updateDialog: (id: number, config: Partial<DialogConfig>) => void;
}

const DialogContext = createContext<DialogContextValue>({
  showDialog: () => 0,
  closeDialog: () => {},
  updateDialog: () => {},
});
export const useDialog = () => useContext(DialogContext);

const iconForType = (type: DialogType) => {
  switch (type) {
    case "success": return <CheckOne fill="#22c55e" size="28" theme="outline" />;
    case "error": return <CloseOne fill="#ef4444" size="28" theme="outline" />;
    case "warning": return <Attention fill="#f59e0b" size="28" theme="outline" />;
    case "question": return <Refresh fill="#0ea5e9" size="28" theme="outline" />;
    case "loading": return <div className="dialog-loading-spinner" />;
    default: return <Info fill="#0ea5e9" size="28" theme="outline" />;
  }
};

const accentForType = (type: DialogType) => {
  switch (type) {
    case "success": return "rgba(34, 197, 94, 0.10)";
    case "error": return "rgba(239, 68, 68, 0.10)";
    case "warning": return "rgba(245, 158, 11, 0.10)";
    case "question": return "rgba(14, 165, 233, 0.10)";
    default: return "rgba(14, 165, 233, 0.06)";
  }
};

/** Parse text with [label](url) markdown links into clickable elements */
const renderMessage = (message: ReactNode) => {
  if (typeof message !== "string") return message;

  const openUrl = (url: string) => {
    open(url);
  };

  const parts: ReactNode[] = [];
  const linkRegex = /\[([^\]]+)\]\(([^)]+)\)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let key = 0;

  while ((match = linkRegex.exec(message)) !== null) {
    if (match.index > lastIndex) {
      parts.push(message.slice(lastIndex, match.index));
    }
    const label = match[1];
    const url = match[2];
    parts.push(
      <a
        key={`link-${key++}`}
        href={url}
        onClick={(e) => {
          e.preventDefault();
          openUrl(url);
        }}
        className="dialog-link"
      >
        {label}
      </a>
    );
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < message.length) {
    parts.push(message.slice(lastIndex));
  }

  return parts.length > 0 ? parts : message;
};

let dialogIdCounter = 0;

export const DialogProvider = ({ children }: { children: ReactNode }) => {
  const [dialogs, setDialogs] = useState<DialogState[]>([]);

  const showDialog = useCallback((config: DialogConfig) => {
    const id = ++dialogIdCounter;
    setDialogs(prev => [...prev, { ...config, id, visible: false }]);
    requestAnimationFrame(() => {
      setDialogs(prev => prev.map(d => d.id === id ? { ...d, visible: true } : d));
    });
    return id;
  }, []);

  const closeDialog = useCallback((id: number, value: string = "ok") => {
    setDialogs(prev => {
      const dlg = prev.find(d => d.id === id);
      setTimeout(() => {
        dlg?.onClose?.(value);
      }, 200);
      return prev.map(d => d.id === id ? { ...d, visible: false } : d);
    });
    setTimeout(() => {
      setDialogs(prev => prev.filter(d => d.id !== id));
    }, 200);
  }, []);

  // 原地更新对话框内容：保留同一个 dialog 元素，只替换标题/消息/按钮等，
  // 避免 close+reopen 造成的视觉跳动。
  const updateDialog = useCallback((id: number, config: Partial<DialogConfig>) => {
    setDialogs(prev => prev.map(d => d.id === id ? { ...d, ...config } : d));
  }, []);

  return (
    <DialogContext.Provider value={{ showDialog, closeDialog, updateDialog }}>
      {children}
      {dialogs.map(d => (
        <div
          key={d.id}
          className={`fixed inset-0 z-[100] flex items-center justify-center transition-all duration-200 ${
            d.visible ? "opacity-100" : "opacity-0"
          }`}
          style={{ background: "rgba(15, 23, 42, 0.2)", backdropFilter: "blur(6px)" }}
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              const lastButton = d.buttons?.[d.buttons.length - 1] || { value: "ok" };
              closeDialog(d.id, lastButton.value);
            }
          }}
        >
          <div
            className={`liquid-dialog transition-all duration-200 ${
              d.visible ? "scale-100 opacity-100" : "scale-95 opacity-0"
            }`}
            style={{
              background: `linear-gradient(145deg, rgba(255,255,255,0.88), rgba(255,255,255,0.75) 55%, ${accentForType(d.type || "info")})`,
            }}
          >
            <div className="flex items-start gap-4 mb-5">
              <div className="flex-shrink-0 mt-0.5">{iconForType(d.type || "info")}</div>
              <div className="flex-1 min-w-0">
                <h3 className="dialog-title">{d.title}</h3>
                <div className="dialog-message whitespace-pre-line leading-relaxed">
                  {renderMessage(d.message)}
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              {(d.buttons || [{ label: "确定", value: "ok", primary: true }]).map(btn => (
                <button
                  key={btn.value}
                  className={`liquid-dialog-btn ${btn.primary ? "liquid-dialog-btn-primary" : ""}`}
                  onClick={() => {
                    if (btn.onClick) {
                      // 原地转换型按钮：不关闭对话框，由调用方决定后续内容
                      btn.onClick(d.id);
                    } else {
                      closeDialog(d.id, btn.value);
                    }
                  }}
                >
                  {btn.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      ))}
    </DialogContext.Provider>
  );
};
