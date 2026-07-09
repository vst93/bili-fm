import { useEffect } from "react";

import ToastContainer from "./components/toast/ToastContainer";

import IndexPage from "@/pages/index";

function App() {
  useEffect(() => {
    // Time-of-day lighting: set data-time-of-day on <html> for subtle sky tint
    const updateTimeOfDay = () => {
      const hour = new Date().getHours();
      let tod: string;
      if (hour >= 5 && hour < 8) tod = "dawn";
      else if (hour >= 8 && hour < 17) tod = "day";
      else if (hour >= 17 && hour < 20) tod = "dusk";
      else tod = "night";
      document.documentElement.setAttribute("data-time-of-day", tod);
    };
    updateTimeOfDay();
    const timer = setInterval(updateTimeOfDay, 60000);

    // ---- Unified pointer tracking (single listener for both card glow + button light) ----
    const cardSelector = '[data-slot="wrapper"] [role="button"][class*="bg-content"]';
    const lightSelector =
      ".liquid-glass-icon-btn, .nav-icon-btn, .top-tool-btn, " +
      ".search-submit-btn, .login-close-btn, .app-title-bar button, " +
      "#switch-window-mode, #switch-window-mode-mini";

    const activeCards = new Set<HTMLElement>();
    const litSurfaces = new Set<HTMLElement>();
    let raf = 0;
    let px = 0;
    let py = 0;
    let pTarget: HTMLElement | null = null;
    let cachedWrapper: HTMLElement | null = null;
    let cachedCards: HTMLElement[] = [];

    const resetCard = (card: HTMLElement) => {
      card.style.removeProperty("--hover-x");
      card.style.removeProperty("--hover-y");
      card.style.removeProperty("--hover-strength");
      card.classList.remove("is-pointer-hovered");
    };

    const clearAll = () => {
      activeCards.forEach(resetCard);
      activeCards.clear();
      litSurfaces.forEach((el) => {
        el.style.removeProperty("--lx");
        el.style.removeProperty("--ly");
      });
      litSurfaces.clear();
      cachedWrapper = null;
      cachedCards = [];
      pTarget = null;
    };

    const tick = () => {
      raf = 0;

      // --- Button light tracking: only process ancestors that match ---
      const nextLit = new Set<HTMLElement>();
      let litNode: Element | null = pTarget;
      while (litNode) {
        if (litNode instanceof HTMLElement && litNode.matches(lightSelector)) {
          const rect = litNode.getBoundingClientRect();
          litNode.style.setProperty(
            "--lx",
            `${Math.round(((px - rect.left) / rect.width) * 1000) / 10}%`,
          );
          litNode.style.setProperty(
            "--ly",
            `${Math.round(((py - rect.top) / rect.height) * 1000) / 10}%`,
          );
          nextLit.add(litNode);
        }
        litNode = litNode.parentElement;
      }
      litSurfaces.forEach((el) => {
        if (!nextLit.has(el)) {
          el.style.removeProperty("--lx");
          el.style.removeProperty("--ly");
        }
      });
      litSurfaces.clear();
      nextLit.forEach((el) => litSurfaces.add(el));

      // --- Card glow: only when inside a drawer wrapper ---
      const wrapper = pTarget?.closest<HTMLElement>("[data-slot=\"wrapper\"]");
      if (!wrapper) return;

      // Cache card list per wrapper to avoid re-querying every frame
      if (cachedWrapper !== wrapper) {
        cachedWrapper = wrapper;
        cachedCards = Array.from(
          wrapper.querySelectorAll<HTMLElement>(cardSelector),
        );
      }
      if (!cachedCards.length) return;

      // Small radius: only the hovered card + its immediate neighbors glow.
      // A large radius lights up dozens of cards simultaneously, each triggering
      // expensive calc()/color-mix()/filter repaints = scroll jank.
      const radius = 180;
      const currentCard = pTarget?.closest<HTMLElement>(cardSelector) ?? null;
      const nextCards = new Set<HTMLElement>();

      for (const card of cachedCards) {
        const rect = card.getBoundingClientRect();
        if (
          rect.bottom < py - radius ||
          rect.top > py + radius ||
          rect.right < px - radius ||
          rect.left > px + radius
        )
          continue;

        const cx = rect.left + rect.width / 2;
        const cy = rect.top + rect.height / 2;
        const dist = Math.hypot(px - cx, py - cy);
        const isCurrent = card === currentCard;
        const strength = isCurrent ? 1 : Math.max(0, 1 - dist / radius) * 0.4;
        if (strength <= 0.1) continue;

        nextCards.add(card);
        card.style.setProperty("--hover-x", `${px - rect.left}px`);
        card.style.setProperty("--hover-y", `${py - rect.top}px`);
        card.style.setProperty("--hover-strength", strength.toFixed(3));
        card.classList.toggle("is-pointer-hovered", isCurrent);
      }

      activeCards.forEach((card) => {
        if (!nextCards.has(card)) resetCard(card);
      });
      activeCards.clear();
      nextCards.forEach((c) => activeCards.add(c));
    };

    const onPointerMove = (event: PointerEvent) => {
      pTarget = event.target as HTMLElement | null;
      px = event.clientX;
      py = event.clientY;
      if (!raf) raf = requestAnimationFrame(tick);
    };

    const onPointerLeave = () => {
      if (raf) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
      clearAll();
    };

    document.addEventListener("pointermove", onPointerMove, { capture: true });
    document.addEventListener("pointerleave", onPointerLeave, true);

    return () => {
      if (raf) cancelAnimationFrame(raf);
      document.removeEventListener("pointermove", onPointerMove, { capture: true });
      document.removeEventListener("pointerleave", onPointerLeave, true);
      clearAll();
      clearInterval(timer);
    };
  }, []);

  return (
    <>
      <IndexPage />
      <ToastContainer />
    </>
  );
}

export default App;
