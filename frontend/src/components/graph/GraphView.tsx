import React, { useCallback, useEffect, useRef, useState } from "react";
import ZoomPinManual from "../zoom-pin-manual";

interface Props {
  svgString: string;
  mainFiles: string[];
  canStep: boolean;
  error: any;
}

const GraphView = ({ svgString, mainFiles, canStep, error }: Props) => {
  const outputRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGElement | null>(null);
  const [zoom, setZoom] = useState<number>(1);
  const [position, setPosition] = useState({ x: 0, y: 50 });
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });

  // add
  const zoomRef = useRef(zoom);
  const positionRef = useRef(position);
  zoomRef.current = zoom;
  positionRef.current = position;

  const applyTransform = useCallback(() => {
    if (svgRef.current) {
      svgRef.current.style.transform = `translate(${position.x}px, ${position.y}px) scale(${zoom})`;
    }
  }, [position.x, position.y, zoom]);

  const render = useCallback(() => {
    if (outputRef.current) {
      try {
        outputRef.current.innerHTML = svgString;

        const svgElement = outputRef.current.querySelector("svg");
        if (svgElement) {
          svgRef.current = svgElement;
          // // 新しい要素に対して、現在のRefの値を使って即座に位置・拡大率を適用
          svgElement.style.transform = `translate(${positionRef.current.x}px, ${positionRef.current.y}px) scale(${zoomRef.current})`;
          svgElement.style.transition = isDragging ? "none" : "transform 0.2s ease";
        }
      } catch (error) {
        outputRef.current.innerHTML = "invalid svg";
      }
    }
  }, [svgString]);

  useEffect(() => {
    render();
  }, [render]);

  useEffect(() => {
    applyTransform();
  }, [position, zoom, applyTransform]);

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();

    const delta = e.deltaY > 0 ? 0.9 : 1.1;

    setZoom((prev) => Math.max(0.1, Math.min(5, prev * delta)));
  };

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      setIsDragging(true);
      setDragStart({ x: e.clientX - position.x, y: e.clientY - position.y });

      if (svgRef.current) {
        svgRef.current.style.transition = "none";
      }
    },
    [position.x, position.y]
  );

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);

    if (svgRef.current) {
      svgRef.current.style.transition = "transform 0.2s ease";
    }
  }, []);

  const handleMouseMove = useCallback(
    (e: React.MouseEvent) => {
      if (isDragging) {
        requestAnimationFrame(() => {
          setPosition({
            x: e.clientX - dragStart.x,
            y: e.clientY - dragStart.y,
          });
        });
      }
    },
    [isDragging, dragStart.x, dragStart.y]
  );

  const popStyle = canStep
    ? "text-center bg-amber-50 py-2 flex flex-col"
    : "text-center bg-red-200 py-2 flex flex-col";

  return (
    <>
      <div className={popStyle}>
        <p>Entry point: {mainFiles.join(", ")}</p>

        <p>{canStep ? "" : "(terminated)"}</p>
      </div>
      <ZoomPinManual />
      {error && (
        <div className="text-center">
          <a className="text-red-400">Error: {error}</a>
        </div>
      )}
      <div
        className="bg-white w-full h-full overflow-hidden"
        onWheel={handleWheel}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        // onMouseLeave={handleMouseUp}
        style={{
          border: "black",
          cursor: isDragging ? "grabbing" : "grab",
          userSelect: "none",
        }}
      >
        <div ref={outputRef} className="w-full h-full" />
        <ZoomPinManual />
      </div>
    </>
  );
};

export default GraphView;
