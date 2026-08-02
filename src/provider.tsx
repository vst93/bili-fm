import { HeroUIProvider } from "@heroui/system";

export function Provider({ children }: { children: React.ReactNode }) {
  return <HeroUIProvider>{children}</HeroUIProvider>;
}
