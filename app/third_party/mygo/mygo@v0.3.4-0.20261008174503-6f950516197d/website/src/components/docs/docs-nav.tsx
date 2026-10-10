import { ArrowUpRightIcon } from "lucide-react"

import { DocLink } from "@/components/docs/doc-link"
import { useActiveHeading } from "@/components/docs/toc"
import type { NavSection, TocItem } from "@/lib/docs"

const item = "flex items-center gap-1 rounded-md px-2.5 py-1.5 text-muted-foreground transition-colors hover:text-foreground"

/** The pages of the docs, with the sections of the current page under it. */
export function DocsNav({ sections, page, onNavigate }: { sections: NavSection[]; page?: { slug: string; toc: TocItem[] }; onNavigate?: () => void }) {
  const headings = page?.toc.filter((t) => t.depth === 2) ?? []
  const active = useActiveHeading(headings)
  return (
    <nav className="flex flex-col gap-6 text-sm">
      {sections.map((section) => (
        <div key={section.title}>
          <h4 className="label mb-2 px-2.5">{section.title}</h4>
          <ul className="flex flex-col gap-px">
            {section.items.map((p) => (
              <li key={p.slug ?? p.href}>
                {p.slug !== undefined ? (
                  <DocLink slug={p.slug} onClick={onNavigate} className={item} activeProps={{ className: "bg-muted font-medium text-foreground!" }}>
                    {p.title}
                  </DocLink>
                ) : (
                  <a href={p.href} className={item}>
                    {p.title}
                    <ArrowUpRightIcon className="size-3 opacity-60" />
                  </a>
                )}
                {p.slug === page?.slug && headings.length > 0 && (
                  <ul className="my-1.5 ml-3.5 border-l">
                    {headings.map((h) => (
                      <li key={h.id}>
                        <a
                          href={`#${h.id}`}
                          onClick={onNavigate}
                          data-active={active === h.id || undefined}
                          className="-ml-px block border-l border-transparent py-1 pr-2 pl-3 text-[13px] leading-5 text-muted-foreground transition-colors hover:text-foreground data-active:border-gopher data-active:text-foreground"
                        >
                          {h.text}
                        </a>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  )
}
