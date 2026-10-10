import { useNavigate } from "@tanstack/react-router"
import * as React from "react"

/** A page rendered from markdown: its links navigate within the site, and its code blocks copy. */
export function DocContent({ html }: { html: string }) {
  const ref = React.useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  React.useEffect(() => {
    const el = ref.current
    if (!el) return
    function onClick(e: MouseEvent) {
      const target = e.target as Element
      const copy = target.closest<HTMLButtonElement>(".copy-button")
      if (copy) {
        const code = copy.parentElement?.querySelector("pre")?.textContent ?? ""
        void navigator.clipboard.writeText(code).then(() => {
          copy.dataset.copied = ""
          setTimeout(() => delete copy.dataset.copied, 1500)
        })
        return
      }
      const a = target.closest("a")
      if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
      const url = new URL(a.href)
      if (url.origin !== location.origin || a.target) return
      e.preventDefault()
      void navigate({ href: url.pathname + url.search + url.hash })
    }
    el.addEventListener("click", onClick)
    return () => el.removeEventListener("click", onClick)
  }, [navigate])

  return <div ref={ref} className="prose-docs" dangerouslySetInnerHTML={{ __html: html }} />
}
