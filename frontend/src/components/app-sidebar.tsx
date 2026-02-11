import { Sidebar, SidebarContent, SidebarHeader } from "@/components/ui/sidebar";
import { EventsOn } from "#wailsjs/runtime";
import { useState, useEffect, useRef } from "react";

const AppSideBar = () => {
  const [logs, setLogs] = useState<string[]>([]);
  const logsEndRef = useRef<null | HTMLDivElement>(null);

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(scrollToBottom, [logs]);

  useEffect(() => {
    const unsubscribeLog = EventsOn("logEvent", (logMessage: string) => {
      setLogs((prevLogs) => [...prevLogs, logMessage]);
    });

    const unsubscribeClear = EventsOn("clearLogs", () => {
      setLogs([]);
    });

    return () => {
      unsubscribeLog();
      unsubscribeClear();
    };
  }, []);

  return (
    <Sidebar>
      <SidebarHeader className="text-white text-xl font-bold">Gousal</SidebarHeader>
      <SidebarContent>
        <div className="h-full flex flex-col">
          <h3 className="text-white text-lg font-semibold p-2 border-b border-gray-600">
            Event Log
          </h3>
          <div className="grow overflow-y-auto py-2">
            {logs.map((log, index) => (
              <div
                key={index}
                className="text-stone-200 px-1 py-2 text-sm font-mono whitespace-break-spaces border-y break-words"
                style={index + 1 == logs.length ? { backgroundColor: "#14B2DD" } : {}}
              >
                {log}
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>
        </div>
      </SidebarContent>
    </Sidebar>
  );
};

export default AppSideBar;
