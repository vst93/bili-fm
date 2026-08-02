export default function DefaultLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="relative flex flex-col h-screen bg-transparent">
      <main className="app-main flex-grow bg-transparent">{children}</main>
    </div>
  );
}
