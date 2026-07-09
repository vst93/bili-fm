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

    const cardSelector = '[data-slot="wrapper"] [role="button"][class*="bg-content"]';
    const activeCards = new Set<HTMLElement>();
    let animationFrame = 0;
    let pointerX = 0;
    let pointerY = 0;
    let pointerTarget: HTMLElement | null = null;

    const resetCard = (card: HTMLElement) => {
      card.style.removeProperty("--hover-x");
      card.style.removeProperty("--hover-y");
      card.style.removeProperty("--hover-strength");
      card.classList.remove("is-pointer-hovered");
    };

    const clearCardGlow = () => {
      activeCards.forEach(resetCard);
      activeCards.clear();
      pointerTarget = null;
    };

    const updateCardGlow = () => {
      animationFrame = 0;
      const radius = 430;
      const currentCard = pointerTarget?.closest<HTMLElement>(cardSelector) ?? null;
      const scope = pointerTarget?.closest<HTMLElement>('[data-slot="wrapper"] > section') ?? document;
      const nextActiveCards = new Set<HTMLElement>();

      scope.querySelectorAll<HTMLElement>(cardSelector).forEach((card) => {
        const rect = card.getBoundingClientRect();

        if (
          rect.bottom < pointerY - radius ||
          rect.top > pointerY + radius ||
          rect.right < pointerX - radius ||
          rect.left > pointerX + radius
        ) {
          return;
        }

        const centerX = rect.left + rect.width / 2;
        const centerY = rect.top + rect.height / 2;
        const distance = Math.hypot(pointerX - centerX, pointerY - centerY);
        const isCurrent = card === currentCard;
        const strength = isCurrent ? 1 : Math.max(0, 1 - distance / radius) * 0.58;

        if (strength <= 0.08) return;

        nextActiveCards.add(card);
        card.style.setProperty("--hover-x", `${pointerX - rect.left}px`);
        card.style.setProperty("--hover-y", `${pointerY - rect.top}px`);
        card.style.setProperty("--hover-strength", strength.toFixed(3));
        card.classList.toggle("is-pointer-hovered", isCurrent);
      });

      activeCards.forEach((card) => {
        if (!nextActiveCards.has(card)) resetCard(card);
      });
      activeCards.clear();
      nextActiveCards.forEach((card) => activeCards.add(card));
    };

    const handlePointerMove = (event: PointerEvent) => {
      pointerTarget = event.target as HTMLElement | null;
      const wrapper = pointerTarget?.closest('[data-slot="wrapper"]');

      if (!wrapper) {
        clearCardGlow();
        return;
      }

      pointerX = event.clientX;
      pointerY = event.clientY;

      if (!animationFrame) {
        animationFrame = requestAnimationFrame(updateCardGlow);
      }
    };

    const handlePointerLeave = () => {
      if (animationFrame) {
        cancelAnimationFrame(animationFrame);
        animationFrame = 0;
      }
      clearCardGlow();
    };

    document.addEventListener("pointermove", handlePointerMove, true);
    document.addEventListener("pointerleave", handlePointerLeave, true);

    return () => {
      if (animationFrame) cancelAnimationFrame(animationFrame);
      document.removeEventListener("pointermove", handlePointerMove, true);
      document.removeEventListener("pointerleave", handlePointerLeave, true);
      clearCardGlow();
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
