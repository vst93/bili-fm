import * as React from "react"

export type Theme = "light" | "dark" | "system"

const key = "theme"
const query = "(prefers-color-scheme: dark)"

/** Runs in <head> before the page paints, so a dark page never flashes light. */
export const themeScript = `try{var t=localStorage.getItem("${key}");document.documentElement.classList.toggle("dark",t==="dark"||(t!=="light"&&matchMedia("${query}").matches))}catch(e){}`

function stored(): Theme {
  try {
    const t = localStorage.getItem(key)
    return t === "light" || t === "dark" ? t : "system"
  } catch {
    return "system"
  }
}

function apply(theme: Theme) {
  const dark = theme === "dark" || (theme === "system" && matchMedia(query).matches)
  const root = document.documentElement
  if (root.classList.contains("dark") === dark) return
  // Switch colors at once, without every transition of the page running.
  const style = document.createElement("style")
  style.textContent = "*,*::before,*::after{transition:none!important}"
  document.head.append(style)
  root.classList.toggle("dark", dark)
  getComputedStyle(root).colorScheme
  requestAnimationFrame(() => style.remove())
}

const listeners = new Set<() => void>()

export function setTheme(theme: Theme) {
  try {
    if (theme === "system") localStorage.removeItem(key)
    else localStorage.setItem(key, theme)
  } catch {}
  apply(theme)
  listeners.forEach((l) => l())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  const media = matchMedia(query)
  const onChange = () => apply(stored())
  media.addEventListener("change", onChange)
  return () => {
    listeners.delete(listener)
    media.removeEventListener("change", onChange)
  }
}

/** The chosen theme; "system" until the page hydrates. */
export function useTheme(): Theme {
  return React.useSyncExternalStore(subscribe, stored, () => "system")
}
