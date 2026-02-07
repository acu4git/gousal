import { createContext, ReactNode, useContext, useRef, useState } from "react";

type LoadingContextType = {
  isLoading: boolean;
  error: string | null;
  show: () => void;
  hide: () => void;
  resetError: () => void;
};

const LoadingContext = createContext<LoadingContextType | null>(null);

const TIMEOUT_MS = 30000;

export const LoadingProvider = ({ children }: { children: ReactNode }) => {
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [count, setCount] = useState<number>(0);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<number | null>(null);

  const startTimer = () => {
    if (timerRef.current != null) return;

    timerRef.current = window.setTimeout(() => {
      setCount(0);
      setError("Process Timeout");
      timerRef.current = null;
    }, TIMEOUT_MS);
  };

  const clearTimer = () => {
    if (timerRef.current != null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  };

  const show = () => {
    setError(null);
    setCount((c) => {
      const next = Math.max(0, c - 1);
      if (next === 0) startTimer();
      return c + 1;
    });
    setIsLoading(true);
  };

  const hide = () => {
    setCount((c) => {
      const next = Math.max(0, c - 1);
      if (next === 0) clearTimer();
      return next;
    });

    setIsLoading(false);
  };

  return (
    <LoadingContext.Provider
      value={{ isLoading: count > 0, error, show, hide, resetError: () => setError(null) }}
    >
      {children}
    </LoadingContext.Provider>
  );
};

export const useLoading = () => {
  const ctx = useContext(LoadingContext);
  if (!ctx) {
    throw new Error("useLoading must be used within LoadingProvider");
  }
  return ctx;
};
