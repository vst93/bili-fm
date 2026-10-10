import type * as React from "react"

import { cn } from "@/lib/utils"

// One "!" for each member of MyGO!!!!!, leaning this way and that, coming in
// 0.3 s apart once the page shows.
const bangs = [
  "text-bang-1 -rotate-9 [--bang-delay:0.3s]",
  "text-bang-2 rotate-6 [--bang-delay:0.6s]",
  "text-bang-3 -rotate-4 [--bang-delay:0.9s]",
  "text-bang-4 rotate-8 [--bang-delay:1.2s]",
  "text-bang-5 -rotate-6 [--bang-delay:1.5s]",
]

/** The name: MyGo, then the five "!" of MyGO!!!!!, in its members' colors. Read as "MyGo". */
export function Wordmark({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span className={cn("font-semibold tracking-tight", className)} {...props}>
      MyGo
      <span aria-hidden="true" className="ml-px font-extrabold tracking-normal">
        {bangs.map((bang) => (
          <span key={bang} className={cn("inline-block motion-safe:animate-bang", bang)}>
            !
          </span>
        ))}
      </span>
    </span>
  )
}
