import { useNavigate } from "@tanstack/react-router"
import { FileTextIcon, HashIcon, SearchIcon } from "lucide-react"
import * as React from "react"

import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import { Kbd } from "@/components/ui/kbd"
import { docHref, type SearchEntry } from "@/lib/docs"

// Prerendered by the /docs/search.json route, and fetched the first time the
// search opens.
let index: Promise<SearchEntry[]> | undefined
function loadIndex() {
  index ??= fetch("/docs/search.json").then((r) => {
    if (!r.ok) throw new Error(`search index: ${r.status}`)
    return r.json() as Promise<SearchEntry[]>
  })
  index.catch(() => (index = undefined))
  return index
}

interface Result {
  entry: SearchEntry
  score: number
}

/** Entries that contain every term, best first: in the heading, then the page's title, then the text. */
function search(entries: SearchEntry[], terms: string[]): Result[] {
  const results: Result[] = []
  for (const entry of entries) {
    const heading = entry.heading.toLowerCase()
    const page = entry.page.toLowerCase()
    const text = entry.text.toLowerCase()
    let score = 0
    for (const term of terms) {
      const s = heading.startsWith(term) ? 12 : heading.includes(term) ? 8 : page.includes(term) ? 3 : text.includes(term) ? 1 : 0
      if (!s) {
        score = 0
        break
      }
      score += s
    }
    if (score) results.push({ entry, score: entry.id ? score : score + 2 })
  }
  return results.sort((a, b) => b.score - a.score).slice(0, 40)
}

function snippet(text: string, terms: string[]) {
  const lower = text.toLowerCase()
  const at = Math.min(...terms.map((t) => lower.indexOf(t)).filter((i) => i >= 0))
  if (!Number.isFinite(at)) return text.slice(0, 140)
  const start = Math.max(0, lower.lastIndexOf(" ", Math.max(0, at - 40)))
  return (start > 0 ? "…" : "") + text.slice(start, start + 160).trim()
}

function Highlight({ text, terms }: { text: string; terms: string[] }) {
  if (!terms.length) return text
  const pattern = new RegExp(`(${terms.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`, "gi")
  return text.split(pattern).map((part, i) =>
    i % 2 ? (
      <mark key={i} className="rounded-sm bg-gopher/20 text-foreground">
        {part}
      </mark>
    ) : (
      part
    )
  )
}

export function Search() {
  const [open, setOpen] = React.useState(false)
  const [mac, setMac] = React.useState(true)

  React.useEffect(() => {
    setMac(/Mac|iPhone|iPad/.test(navigator.platform))
    function onKeyDown(e: KeyboardEvent) {
      const typing = e.target instanceof HTMLElement && (e.target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName))
      if ((e.key === "k" && (e.metaKey || e.ctrlKey)) || (e.key === "/" && !typing)) {
        e.preventDefault()
        setOpen((o) => !o)
      }
    }
    document.addEventListener("keydown", onKeyDown)
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [])

  return (
    <>
      <Button
        variant="outline"
        onClick={() => setOpen(true)}
        onPointerEnter={() => void loadIndex().catch(() => {})}
        className="hidden h-8 w-56 justify-start gap-2 bg-muted/40 px-2.5 font-normal text-muted-foreground shadow-none hover:bg-muted md:flex dark:bg-muted/30"
      >
        <SearchIcon />
        Search docs…
        <Kbd className="ml-auto border bg-background">{mac ? "⌘K" : "Ctrl K"}</Kbd>
      </Button>
      <Button variant="ghost" size="icon" onClick={() => setOpen(true)} className="md:hidden" aria-label="Search docs">
        <SearchIcon />
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent showCloseButton={false} className="top-[12%] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-xl">
          <DialogTitle className="sr-only">Search the docs</DialogTitle>
          <DialogDescription className="sr-only">Find a page or a section of the docs.</DialogDescription>
          {open && <SearchPanel onClose={() => setOpen(false)} />}
        </DialogContent>
      </Dialog>
    </>
  )
}

function SearchPanel({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate()
  const [query, setQuery] = React.useState("")
  const [entries, setEntries] = React.useState<SearchEntry[]>()
  const [failed, setFailed] = React.useState(false)

  React.useEffect(() => {
    loadIndex().then(setEntries, () => setFailed(true))
  }, [])

  const terms = React.useMemo(() => query.toLowerCase().split(/\s+/).filter(Boolean), [query])
  const groups = React.useMemo(() => {
    if (!entries) return []
    const results = terms.length ? search(entries, terms) : entries.filter((e) => !e.id).map((entry) => ({ entry, score: 0 }))
    const bySlug = new Map<string, Result[]>()
    for (const r of results) bySlug.set(r.entry.slug, [...(bySlug.get(r.entry.slug) ?? []), r])
    return [...bySlug.values()]
  }, [entries, terms])

  function go(entry: SearchEntry) {
    onClose()
    void navigate({ href: docHref(entry.slug, entry.id) })
  }

  return (
    <Command shouldFilter={false} className="rounded-none! bg-transparent p-0">
      <div className="border-b p-1.5">
        <CommandInput value={query} onValueChange={setQuery} placeholder="Search the docs…" className="h-9" />
      </div>
      <CommandList className="max-h-[min(60vh,26rem)] p-1.5">
        {failed ? (
          <p className="py-10 text-center text-sm text-muted-foreground">The search index could not be loaded.</p>
        ) : !entries ? (
          <p className="py-10 text-center text-sm text-muted-foreground">Loading…</p>
        ) : (
          <CommandEmpty className="py-10 text-muted-foreground">No results for “{query}”</CommandEmpty>
        )}
        {groups.map((results) => (
          <CommandGroup key={results[0]!.entry.slug} heading={terms.length ? results[0]!.entry.page : undefined}>
            {results.map(({ entry }) => (
              <CommandItem
                key={`${entry.slug}#${entry.id}`}
                value={`${entry.slug}#${entry.id}`}
                onSelect={() => go(entry)}
                className="items-start gap-3 py-2"
              >
                {entry.id ? <HashIcon className="mt-0.5 text-muted-foreground" /> : <FileTextIcon className="mt-0.5 text-muted-foreground" />}
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">
                    <Highlight text={entry.heading} terms={terms} />
                  </div>
                  {terms.length > 0 && entry.text && (
                    <div className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted-foreground">
                      <Highlight text={snippet(entry.text, terms)} terms={terms} />
                    </div>
                  )}
                </div>
              </CommandItem>
            ))}
          </CommandGroup>
        ))}
      </CommandList>
    </Command>
  )
}
