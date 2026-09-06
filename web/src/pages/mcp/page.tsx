import { useTranslation } from 'react-i18next'
import { useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import MCPList from "@/pages/mcp/components/MCPList";
import EmbedMCP from "@/pages/mcp/components/EmbedMCP";
import MCPConfig from "@/pages/mcp/components/MCPConfig";

const MCPPage = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState("list");

  return (
    <div className="mx-auto flex min-h-full w-full max-w-[1800px] flex-col gap-4 p-4 sm:p-6">
      <div>
        <h1 className="text-lg font-semibold">{t("ui.mcpTitle")}</h1>

      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid h-10 w-full max-w-lg grid-cols-3">
          <TabsTrigger value="list" >{t("ui.mcpList")}</TabsTrigger>
          <TabsTrigger value="embed" >{t("ui.mcpEmbed")}</TabsTrigger>
          <TabsTrigger value="config" >{t("ui.mcpConfig")}</TabsTrigger>
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
