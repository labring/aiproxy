import { useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import MCPList from "@/pages/mcp/components/MCPList";
import EmbedMCP from "@/pages/mcp/components/EmbedMCP";
import MCPConfig from "@/pages/mcp/components/MCPConfig";

const MCPPage = () => {
  const [activeTab, setActiveTab] = useState("list");

  return (
    <div className="mx-auto flex h-full w-full max-w-[1800px] flex-col gap-5 p-4 sm:p-6 lg:p-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">MCP 管理</h1>
        <p className="mt-1 text-sm text-muted-foreground">配置、嵌入并管理消息控制协议服务</p>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid h-auto w-full max-w-xl grid-cols-3 rounded-xl bg-muted/60 p-1">
          <TabsTrigger value="list" className="rounded-lg py-2.5">服务列表</TabsTrigger>
          <TabsTrigger value="embed" className="rounded-lg py-2.5">嵌入服务</TabsTrigger>
          <TabsTrigger value="config" className="rounded-lg py-2.5">配置管理</TabsTrigger>
        </TabsList>

        <TabsContent value="list">
          <MCPList />
        </TabsContent>

        <TabsContent value="embed">
          <EmbedMCP />
        </TabsContent>

        <TabsContent value="config">
          <MCPConfig />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default MCPPage;
