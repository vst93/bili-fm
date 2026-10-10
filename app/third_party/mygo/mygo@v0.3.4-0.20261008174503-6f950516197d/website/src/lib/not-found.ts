declare global {
  interface Window {
    /** The path that 404.html is served at. */
    __notFoundPath?: string
  }
}

/** Run by 404.html before the page hydrates. */
export const notFoundScript = "window.__notFoundPath=location.pathname"
