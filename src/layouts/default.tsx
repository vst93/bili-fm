import type { CSSProperties } from "react";

export default function DefaultLayout({
  children,
  ambientCover,
}: {
  children: React.ReactNode;
  ambientCover?: string;
}) {
  const style = ambientCover
    ? ({
        "--ambient-cover": `url(${JSON.stringify(ambientCover)})`,
      } as CSSProperties)
    : undefined;

  return (
    <div className="app-shell relative flex flex-col h-screen bg-transparent" style={style}>
      <main className="app-main flex-grow bg-transparent">{children}</main>
    </div>
  );
}
