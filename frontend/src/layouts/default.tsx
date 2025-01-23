export default function DefaultLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="relative flex flex-col h-screen bg-transparent">
      <main className="container mx-auto max-w-7xl px-6 flex-grow pt-3 bg-transparent">
        {children}
      </main>
    </div>
  );
}
