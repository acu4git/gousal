import { useLoading } from "@/context/loading/LoadingContext";
import Layout from "@/layout";
import View from "@/pages/View";
import { HashRouter, Route, Routes } from "react-router-dom";

const AppRouter = () => {
  const ctx = useLoading();
  return (
    <HashRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<View />} />
        </Route>
      </Routes>
    </HashRouter>
  );
};

export default AppRouter;
