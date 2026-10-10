import { Link } from "@tanstack/react-router"

import { site } from "@/lib/site"

export function SiteFooter() {
  return (
    <footer className="frame label flex flex-wrap items-center justify-between gap-4 border-t px-4 py-6 sm:px-6">
      <span>
        MyGo {__MYGO_VERSION__} ·{" "}
        <a href={`${site.repo}/blob/main/LICENSE`} className="hover:text-foreground">
          MIT License
        </a>
      </span>
      <nav className="flex gap-6">
        <Link to="/docs" className="hover:text-foreground">
          Docs
        </Link>
        <Link to="/benchmarks" className="hover:text-foreground">
          Benchmarks
        </Link>
        <a href={`${site.repo}/releases`} className="hover:text-foreground">
          Releases
        </a>
        <a href={site.repo} className="hover:text-foreground">
          GitHub
        </a>
      </nav>
    </footer>
  )
}
