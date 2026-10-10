import { Link } from "@tanstack/react-router"

import { buttonVariants } from "@/components/ui/button"

export function NotFound() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center px-6 py-32 text-center">
      <p className="font-mono text-sm text-muted-foreground">404</p>
      <h1 className="mt-3 text-3xl font-semibold tracking-tight">This page does not exist</h1>
      <p className="mt-3 max-w-sm text-muted-foreground">
        It may have moved. The docs, or their search, will have what you are looking for.
      </p>
      <div className="mt-8 flex gap-3">
        <Link to="/docs" className={buttonVariants({ size: "lg" })}>
          Read the docs
        </Link>
        <Link to="/" className={buttonVariants({ variant: "outline", size: "lg" })}>
          Home
        </Link>
      </div>
    </main>
  )
}
