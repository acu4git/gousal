import { Button } from "@/components/ui/button";

interface Props {
  selectGoProject: () => Promise<void>;
  step: () => Promise<void>;
  handleClickAuto: () => Promise<void>;
  isReady: boolean;
  isTerm: boolean;
  isAuto: boolean;
}

const Toolbar = ({ selectGoProject, step, handleClickAuto, isReady, isTerm, isAuto }: Props) => {
  return (
    <div className="text-center h-10 border-b flex items-center px-2 pb-2 justify-between">
      <Button
        onClick={selectGoProject}
        className="bg-gray-300 text-gray-900 hover:cursor-pointer hover:bg-gray-200"
      >
        Select Go Project
      </Button>
      <div className="flex gap-3">
        <Button
          className="bg-blue-600 hover:cursor-pointer hover:bg-blue-500"
          onClick={step}
          disabled={!isReady || isTerm || isAuto}
        >
          Next
          {/* <MdNextPlan /> */}
        </Button>
        {isAuto ? (
          <Button
            className="bg-red-600 hover:cursor-pointer hover:bg-red-500"
            disabled={!isReady || isTerm}
            onClick={handleClickAuto}
          >
            Stop
            {/* <IoPlayCircle /> */}
          </Button>
        ) : (
          <Button
            className="bg-green-600 hover:cursor-pointer hover:bg-green-500"
            disabled={!isReady || isTerm}
            onClick={handleClickAuto}
          >
            Auto Play
            {/* <IoPlayCircle /> */}
          </Button>
        )}
      </div>
    </div>
  );
};

export default Toolbar;
