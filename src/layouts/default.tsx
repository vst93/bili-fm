import { useEffect, useState, type CSSProperties } from "react";

export default function DefaultLayout({
  children,
  ambientCover,
  premiumTexture = true,
}: {
  children: React.ReactNode;
  ambientCover?: string;
  premiumTexture?: boolean;
}) {
  // 封面背景「先预载、后换源」：切歌时旧背景一直画在屏上，新图加载完成
  // 才更新 CSS 变量。否则背景图 URL 立即换掉，新图未就绪时整窗背景会闪一下空白。
  const [appliedCover, setAppliedCover] = useState("");

  useEffect(() => {
    if (!ambientCover) return;
    let cancelled = false;
    const img = new Image();
    img.onload = () => {
      if (!cancelled) setAppliedCover(ambientCover);
    };
    // 加载失败保持旧背景，同样不闪
    img.src = ambientCover;
    return () => {
      cancelled = true;
    };
  }, [ambientCover]);

  const style = appliedCover
    ? ({ "--ambient-cover": `url(${JSON.stringify(appliedCover)})` } as CSSProperties)
    : undefined;
  // 开/关走 opacity 渐变：关闭时旧图仍画在层里只是透明度归零，
  // 首次开启也是等新图就绪后随渐显出现，两端都不会闪。
  const className = `app-shell relative flex flex-col h-screen bg-transparent${
    appliedCover && ambientCover ? "" : " ambient-off"
  }${premiumTexture ? "" : " texture-basic"}`;

  return (
    <div className={className} style={style}>
      <main className="app-main flex-grow bg-transparent">{children}</main>
    </div>
  );
}
