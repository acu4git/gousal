import { createContext, ReactNode, useContext, useState } from "react";

type LoadingContextType = {
  isLoading: boolean;
  show: () => void;
  hide: () => void;
};

const LoadingContext = createContext<LoadingContextType | null>(null);

export const LoadingProvider = ({ children }: { children: ReactNode }) => {
  const [isLoading, setIsLoading] = useState<boolean>(false);

  const show = () => setIsLoading(true);
  const hide = () => setIsLoading(false);

  return (
    <LoadingContext.Provider value={{ isLoading, show, hide }}>{children}</LoadingContext.Provider>
  );
};

export const useLoading = () => {
  const ctx = useContext(LoadingContext);
  if (!ctx) {
    throw new Error("useLoading must be used within LoadingProvider");
  }
  return ctx;
};
