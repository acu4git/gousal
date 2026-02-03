import { Button } from "@/components/ui/button";
import { useLoading } from "@/context/loading/LoadingContext";

export const LoadingOverlay = () => {
  const { isLoading, error, resetError } = useLoading();

  if (!isLoading && !error) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      {error ? (
        <div className="rounded-xl bg-white p-6 text-center shadow-xl">
          <p className="mb-4 text-red-600 font-semibold">{error}</p>
          <Button
            onClick={resetError}
            className="rounded px-4 py-2 text-white hover:cursor-pointer bg-blue-500 hover:bg-blue-400"
          >
            Close
          </Button>
        </div>
      ) : (
        <div className="h-12 w-12 animate-spin rounded-full border-4 border-white/30 border-t-white" />
      )}
    </div>
  );
};
