import { Sidebar, SidebarContent, SidebarHeader } from "@/components/ui/sidebar";

const AppSideBar = () => {
  return (
    <Sidebar>
      <SidebarHeader className="text-white text-xl font-bold">Go Vizualizar</SidebarHeader>
      <SidebarContent />
    </Sidebar>
  );
};

export default AppSideBar;
