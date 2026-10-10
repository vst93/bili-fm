import * as React from "react"

import type { TocItem } from "@/lib/docs"

/** The heading that the reader is at: the last one above the top of the window. */
export function useActiveHeading(items: TocItem[]) {
  const [active, setActive] = React.useState<string>()
  React.useEffect(() => {
    let frame = 0
    function update() {
      frame = 0
      let current: string | undefined
      for (const item of items) {
        const top = document.getElementById(item.id)?.getBoundingClientRect().top
        if (top === undefined || top > 140) break
        current = item.id
      }
      const bottom = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4
      setActive(bottom && current ? items.at(-1)?.id : current)
    }
    function onScroll() {
      frame ||= requestAnimationFrame(update)
    }
    update()
    window.addEventListener("scroll", onScroll, { passive: true })
    return () => {
      window.removeEventListener("scroll", onScroll)
      cancelAnimationFrame(frame)
    }
  }, [items])
  return active
}
