import { useEffect } from "react";

import ToastContainer from "./components/toast/ToastContainer";
import { DialogProvider } from "./components/dialog/DialogProvider";

import IndexPage from "@/pages/index";

function App() {
  useEffect(() => {
    // Time-of-day lighting: richer schedule with 11 segments.
    // 0-6h is calmer (2 states), daytime changes more frequently (3 states).
    // All backgrounds are kept light enough (L > 65%) for gray #64748b icons.
    const updateTimeOfDay = () => {
      const h = new Date().getHours();
      let tod: string;
      if (h < 3) tod = "midnight";       // 0-3: deep night, calm
      else if (h < 6) tod = "predawn";   // 3-6: pre-dawn, still calm
      else if (h < 7) tod = "dawn";      // 6-7: sunrise transition
      else if (h < 9) tod = "morning";   // 7-9: warm morning light
      else if (h < 12) tod = "midday";   // 9-12: bright midday
      else if (h < 14) tod = "noon";     // 12-14: peak brightness
      else if (h < 17) tod = "afternoon";// 14-17: warm afternoon
      else if (h < 18) tod = "golden";   // 17-18: golden hour
      else if (h < 20) tod = "dusk";     // 18-20: sunset
      else if (h < 22) tod = "evening";  // 20-22: blue evening
      else tod = "latenight";            // 22-24: late night
      document.documentElement.setAttribute("data-time-of-day", tod);
    };
    updateTimeOfDay();
    const timer = setInterval(updateTimeOfDay, 60000);

    // Pointer tracking is limited to the hovered drawer card. Button highlights
    // use static CSS states to avoid layout reads on every pointer move.
    const cardSelector = '[data-slot="wrapper"] [role="button"][class*="bg-content"]';

    let activeCard: HTMLElement | null = null;
    let raf = 0;
    let px = 0;
    let py = 0;
    const pointerEffectsEnabled = window.matchMedia(
      "(pointer: fine) and (prefers-reduced-motion: no-preference)",
    );

    const resetCard = (card: HTMLElement) => {
      card.style.removeProperty("--hover-x");
      card.style.removeProperty("--hover-y");
      card.style.removeProperty("--hover-strength");
      card.classList.remove("is-pointer-hovered");
    };

    const clearAll = () => {
      if (activeCard) resetCard(activeCard);
      activeCard = null;
    };

    const tick = () => {
      raf = 0;

      if (!activeCard) return;
      const rect = activeCard.getBoundingClientRect();
      activeCard.style.setProperty("--hover-x", `${px - rect.left}px`);
      activeCard.style.setProperty("--hover-y", `${py - rect.top}px`);
      activeCard.style.setProperty("--hover-strength", "1");
      activeCard.classList.add("is-pointer-hovered");
    };

    const onCardPointerMove = (event: PointerEvent) => {
      if (!pointerEffectsEnabled.matches || document.body.classList.contains("platform-linux")) {
        clearAll();
        return;
      }
      px = event.clientX;
      py = event.clientY;
      if (!raf) raf = requestAnimationFrame(tick);
    };

    const stopTracking = () => {
      if (raf) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
      activeCard?.removeEventListener("pointermove", onCardPointerMove);
      clearAll();
    };

    const onPointerOver = (event: PointerEvent) => {
      if (!pointerEffectsEnabled.matches || document.body.classList.contains("platform-linux")) return;
      const card = (event.target as HTMLElement | null)?.closest<HTMLElement>(cardSelector) ?? null;
      if (!card || card === activeCard) return;
      stopTracking();
      activeCard = card;
      activeCard.addEventListener("pointermove", onCardPointerMove, { passive: true });
      onCardPointerMove(event);
    };

    const onPointerOut = (event: PointerEvent) => {
      if (!activeCard) return;
      const nextTarget = event.relatedTarget as Node | null;
      if (nextTarget && activeCard.contains(nextTarget)) return;
      stopTracking();
    };

    document.addEventListener("pointerover", onPointerOver, { passive: true });
    document.addEventListener("pointerout", onPointerOut, { passive: true });

    return () => {
      document.removeEventListener("pointerover", onPointerOver);
      document.removeEventListener("pointerout", onPointerOut);
      stopTracking();
      clearInterval(timer);
    };
  }, []);

  return (
    <DialogProvider>
      <IndexPage />
      <ToastContainer />
    </DialogProvider>
  );
}

export default App;
