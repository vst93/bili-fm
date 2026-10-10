import { Link, type LinkComponentProps } from "@tanstack/react-router"

/**
 * A link to a page of the docs by its slug, "" for the introduction. Only
 * the page itself is active: /docs/plugins is not when /docs/plugins/fetch is.
 */
export function DocLink({ slug, ...props }: { slug: string } & Omit<LinkComponentProps<"a">, "to" | "params">) {
  return slug ? (
    <Link to="/docs/$" params={{ _splat: slug }} activeOptions={{ exact: true }} {...props} />
  ) : (
    <Link to="/docs" activeOptions={{ exact: true }} {...props} />
  )
}
