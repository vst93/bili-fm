import { CheckIcon, CopyIcon } from "lucide-react"
import * as React from "react"

import { cn } from "@/lib/utils"

export function InstallCommand({ command = "bunx mygo-cli init my-app", className }: { command?: string; className?: string }) {
  const [copied, setCopied] = React.useState(false)
  return (
    <button
      type="button"
      aria-label={`Copy "${command}"`}
      onClick={() =>
        void navigator.clipboard.writeText(command).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 1500)
        })
      }
      className={cn(
        "group inline-flex h-11 max-w-full items-center gap-3 rounded-lg border bg-card pr-3 pl-4 font-mono text-[13.5px] transition-colors hover:border-foreground/25",
        className
      )}
    >
      <span className="text-gopher-ink select-none">$</span>
      <span className="min-w-0 truncate">{command}</span>
      <span className="ml-2 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground">
        {copied ? <CheckIcon className="size-4" /> : <CopyIcon className="size-4" />}
      </span>
    </button>
  )
}
