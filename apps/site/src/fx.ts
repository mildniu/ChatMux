import { useEffect, useRef, useState } from "react";

/* Shared motion utilities for the marketing site. Every effect is
   transform/opacity-only and disabled under prefers-reduced-motion. */

export function prefersReducedMotion(): boolean {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/* Pointer spotlight: exposes --spot-x/--spot-y (viewport px) on <html>
   so fixed atmosphere layers can follow the cursor. */
export function usePointerGlow() {
  useEffect(() => {
    if (prefersReducedMotion()) return;
    const root = document.documentElement;
    let raf = 0;
    let x = window.innerWidth / 2;
    let y = window.innerHeight / 3;
    const paint = () => {
      root.style.setProperty("--spot-x", `${x}px`);
      root.style.setProperty("--spot-y", `${y}px`);
      raf = 0;
    };
    const onMove = (e: PointerEvent) => {
      x = e.clientX;
      y = e.clientY;
      if (!raf) raf = requestAnimationFrame(paint);
    };
    paint();
    window.addEventListener("pointermove", onMove, { passive: true });
    return () => {
      window.removeEventListener("pointermove", onMove);
      cancelAnimationFrame(raf);
    };
  }, []);
}

/* Scroll progress (0..1) as --scroll-p on <html>, plus a `scrolled`
   boolean for the shrinking nav. */
export function useScrollFx(): boolean {
  const [scrolled, setScrolled] = useState(false);
  useEffect(() => {
    const root = document.documentElement;
    let raf = 0;
    const paint = () => {
      const max = root.scrollHeight - window.innerHeight;
      const p = max > 0 ? window.scrollY / max : 0;
      root.style.setProperty("--scroll-p", p.toFixed(4));
      setScrolled(window.scrollY > 10);
      raf = 0;
    };
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(paint);
    };
    paint();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      cancelAnimationFrame(raf);
    };
  }, []);
  return scrolled;
}

/* Card spotlight: one delegated listener keeps a local --mx/--my on any
   `.glow-card` under the pointer, driving its border/inner glow. */
export function useCardSpotlight() {
  useEffect(() => {
    if (prefersReducedMotion()) return;
    const onMove = (e: PointerEvent) => {
      const card = (e.target as HTMLElement | null)?.closest?.(".glow-card");
      if (!(card instanceof HTMLElement)) return;
      const r = card.getBoundingClientRect();
      card.style.setProperty("--mx", `${e.clientX - r.left}px`);
      card.style.setProperty("--my", `${e.clientY - r.top}px`);
    };
    document.addEventListener("pointermove", onMove, { passive: true });
    return () => document.removeEventListener("pointermove", onMove);
  }, []);
}

/* 3D tilt: rotates the element a few degrees toward the pointer. */
export function useTilt<T extends HTMLElement>(maxDeg = 5) {
  const ref = useRef<T | null>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el || prefersReducedMotion()) return;
    if (!window.matchMedia("(pointer: fine)").matches) return;
    let raf = 0;
    const onMove = (e: PointerEvent) => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        const r = el.getBoundingClientRect();
        const px = (e.clientX - r.left) / r.width - 0.5;
        const py = (e.clientY - r.top) / r.height - 0.5;
        el.style.setProperty("--tilt-x", `${(-py * maxDeg).toFixed(2)}deg`);
        el.style.setProperty("--tilt-y", `${(px * maxDeg).toFixed(2)}deg`);
      });
    };
    const onLeave = () => {
      cancelAnimationFrame(raf);
      el.style.setProperty("--tilt-x", "0deg");
      el.style.setProperty("--tilt-y", "0deg");
    };
    el.addEventListener("pointermove", onMove);
    el.addEventListener("pointerleave", onLeave);
    return () => {
      el.removeEventListener("pointermove", onMove);
      el.removeEventListener("pointerleave", onLeave);
      cancelAnimationFrame(raf);
    };
  }, [maxDeg]);
  return ref;
}

/* Active nav link: highlights the section currently in view. */
export function useActiveSection(ids: string[]): string {
  const [active, setActive] = useState("");
  useEffect(() => {
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) setActive(e.target.id);
        }
      },
      { rootMargin: "-30% 0px -60% 0px" },
    );
    for (const id of ids) {
      const el = document.getElementById(id);
      if (el) io.observe(el);
    }
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ids.join(",")]);
  return active;
}
