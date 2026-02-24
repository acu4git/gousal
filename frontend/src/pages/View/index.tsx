import { useCallback, useEffect, useState } from "react";
import { SelectGoProject, Step, Trace } from "#wailsjs/go/main/App";
import GraphView from "@/components/graph/GraphView";
import Toolbar from "@/components/app-toolbar";
import FileSelector from "@/components/file-selector";

const View = () => {
  const [isReady, setIsReady] = useState<boolean>(false);
  const [isAuto, setIsAuto] = useState<boolean>(false);
  const [canStep, setCanStep] = useState<boolean>(false);
  const [projectSelected, setProjectSelected] = useState<boolean>(false);

  const [mainFiles, setMainFiles] = useState<string[]>([]);
  const [mainRecords, setMainRecords] = useState<Record<string, string[]>>({});
  const [svgString, setSvgString] = useState<string>("");
  const [error, setError] = useState<any>(null);

  const selectGoProject = async () => {
    try {
      const res = await SelectGoProject();
      setMainRecords(res);
    } catch (e) {
      console.log(e);
      setError(e);
      return;
    }
    setMainFiles([]);
    setSvgString("");
    setProjectSelected(true);
    setIsReady(false);
    setIsAuto(false);
    setCanStep(false);
    setError(null);
  };

  const trace = async (files: string[]) => {
    try {
      const initSVG = await Trace(files);
      setIsReady(true);
      setCanStep(true);
      setSvgString(initSVG);
    } catch (e) {
      console.log(e);
      setError(e);
      return;
    }
  };

  const step = useCallback(async () => {
    try {
      const res = await Step();
      setCanStep(res.canStep);
      setSvgString(res.svg);
    } catch (e) {
      console.log(e);
      setError(e);
      return;
    }
  }, []);

  useEffect(() => {
    if (!isAuto || !canStep) return;

    const res = setInterval(() => {
      step();
    }, 200);

    return () => clearInterval(res);
  }, [isAuto, canStep, step]);

  const handleClickAuto = async () => {
    const flag = !isAuto;
    setIsAuto(flag);
    // await autoPlay(flag);
  };

  return (
    // <div className="w-full text-center">
    <div className="flex-1 h-full flex-col">
      <Toolbar
        selectGoProject={selectGoProject}
        step={step}
        handleClickAuto={handleClickAuto}
        isReady={isReady}
        canStep={canStep}
        isAuto={isAuto}
      />

      {/* Graph View */}
      {mainFiles.length ? (
        <GraphView mainFiles={mainFiles} svgString={svgString} canStep={canStep} error={error} />
      ) : projectSelected ? (
        <FileSelector mainRecords={mainRecords} setMainFiles={setMainFiles} trace={trace} />
      ) : (
        <div className="flex items-center justify-center h-full">
          <p>No Project Selected</p>
        </div>
      )}
    </div>
  );
};

export default View;
