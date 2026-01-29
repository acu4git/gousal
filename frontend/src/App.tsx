import { LoadingProvider } from "@/context/loading/LoadingContext";
import { LoadingOverlay } from "@/context/loading/LoadingOverlay";
import AppRouter from "@/router";

const App = () => {
  return (
    <LoadingProvider>
      <LoadingOverlay />
      <AppRouter />
    </LoadingProvider>
  );
};

export default App;
