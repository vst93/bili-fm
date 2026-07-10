import { useState, createContext, useContext, useCallback } from "react";
import { CheckOne, CloseOne, Attention, Info, Refresh } from "@icon-park/react";

type DialogType = "info" | "success" | "warning" | "error" | "question";

interface DialogButton {
  label: string;
  value: string;
  primary?: boolean;
}

interface DialogConfig {
  title: string;
  message: string;
  type?: DialogType;
  buttons?: DialogButton[];
  onClose?: (value: string) => void;
}

interface DialogState extends DialogConfig {
  id: number;
  visible: boolean;
}

const DialogContext = createContext<(config: DialogConfig) => void>(() => {});

export const useDialog = () => useContext(DialogContext);

const iconForType = (type: DialogType) => {
  switch (type) {
    case "success":
      return <CheckOne fill="#22c55e" size="28" theme="outline" />;
    case "error":
      return <CloseOne fill="#f43f5e" size="28" theme="outline" />;
    case "warning":
      return <Attention fill="#f59e0b" size="28" theme="outline" />;
    case "question":
      return <Refresh fill="#0ea5e9" size="28" theme="outline" />;
    default:
      return <Info fill="#0ea5e9" size="28" theme="outline" />;
  }
};

const accentForType = (type: DialogType) => {
  switch (type) {
    case "success": return "rgba(34, 197, 94, 0.12)";
    case "error": return "rgba(244, 63, 94, 0.12)";
    case "warning": return "rgba(245, 158, 11, 0.12)";
    case "question": return "rgba(14, 165, 233, 0.12)";
    default: return "rgba(14, 165, 233, 0.08)";
  }
};

let dialogIdCounter = 0;

export const DialogProvider = ({ children }: { children: React.ReactNode }) => {
  const [dialogs, setDialogs] = useState<DialogState[]>([]);

  const showDialog = useCallback((config: DialogConfig) => {
    const id = ++dialogIdCounter;
    setDialogs(prev => [...prev, { ...config, id, visible: false }]);

    // Trigger enter animation on next frame
    requestAnimationFrame(() => {
      setDialogs(prev => prev.map(d => d.id === id ? { ...d, visible: true } : d));
    });
  }, []);

  const closeDialog = useCallback((id: number, value: string) => {
    setDialogs(prev => prev.map(d => d.id === id ? { ...d, visible: false } : d));
    const dialog = dialogs.find(d => d.id === id);
    setTimeout(() => {
      setDialogs(prev => prev.filter(d => d.id !== id));
      dialog?.onClose?.(value);
    }, 200);
  }, [dialogs]);

  return (
    <DialogContext.Provider value={showDialog}>
      {children}
      {dialogs.map(d => (
        <div
          key={d.id}
          className={`fixed inset-0 z-[100] flex items-center justify-center transition-all duration-200 ${
            d.visible ? "opacity-100" : "opacity-0"
          }`}
          style={{ background: "rgba(15, 23, 42, 0.25)", backdropFilter: "blur(4px)" }}
          onClick={(e) => {
            if (e.target === e.currentTarget && d.buttons && d.buttons.length > 1) {
              closeDialog(d.id, d.buttons[d.buttons.length - 1].value);
            }
          }}
        >
          <div
            className={`liquid-dialog transition-all duration-200 ${
              d.visible ? "scale-100 opacity-100" : "scale-95 opacity-0"
            }`}
            style={{
              background: `linear-gradient(145deg, rgba(255,255,255,0.85), rgba(255,255,255,0.7) 60%, ${accentForType(d.type || "info")})`,
            }}
          >
            <div className="flex items-start gap-4 mb-4">
              <div className="flex-shrink-0 mt-0.5">{iconForType(d.type || "info")}</div>
              <div className="flex-1 min-w-0">
                <h3 className="text-base font-semibold text-slate-800 mb-1.5">{d.title}</h3>
                <p className="text-sm text-slate-600 whitespace-pre-line leading-relaxed">{d.message}</p>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              {(d.buttons || [{ label: "确定", value: "ok", primary: true }]).map(btn => (
                <button
                  key={btn.value}
                  className={`liquid-dialog-btn ${btn.primary ? "liquid-dialog-btn-primary" : ""}`}
                  onClick={() => closeDialog(d.id, btn.value)}
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
