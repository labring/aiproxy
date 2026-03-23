import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/use-toast";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { EmbedMCP, getEmbedMCPs, saveEmbedMCP } from "@/api/mcp";
import { useTranslation } from "react-i18next";

const EmbedMCPComponent = () => {
  const [embedMCPs, setEmbedMCPs] = useState<EmbedMCP[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [configValues, setConfigValues] = useState<
    Record<string, Record<string, string>>
  >({});
  const [savingId, setSavingId] = useState<string | null>(null);
  const { toast } = useToast();
  const { t } = useTranslation();

  useEffect(() => {
    fetchEmbedMCPs();
  }, []);

  const fetchEmbedMCPs = async () => {
    try {
      setLoading(true);
      const data = await getEmbedMCPs();
      setEmbedMCPs(data);

      // Initialize config values
      const initialConfigValues: Record<string, Record<string, string>> = {};
      data.forEach((mcp) => {
        initialConfigValues[mcp.id] = {};
        Object.entries(mcp.config_templates).forEach(([key, template]) => {
          initialConfigValues[mcp.id][key] = template.example || "";
        });
      });
      setConfigValues(initialConfigValues);
    } catch {
      toast({
        title: t("error.loading"),
        description: t("mcp.embed.noEmbeddedServers"),
        variant: "destructive",
      });
    } finally {
      setLoading(false);
    }
  };

  const handleInputChange = (
    mcpId: string,
    configKey: string,
    value: string
  ) => {
    setConfigValues((prev) => ({
      ...prev,
      [mcpId]: {
        ...prev[mcpId],
        [configKey]: value,
      },
    }));
  };

  const handleStatusToggle = (mcpId: string, enabled: boolean) => {
    setEmbedMCPs((prev) =>
      prev.map((mcp) =>
        mcp.id === mcpId ? { ...mcp, enabled: !enabled } : mcp
      )
    );
  };

  const handleSave = async (mcp: EmbedMCP) => {
    try {
      setSavingId(mcp.id);
      await saveEmbedMCP({
        id: mcp.id,
        enabled: mcp.enabled,
        init_config: configValues[mcp.id] || {},
      });
      toast({
        title: t("common.success"),
        description: `${mcp.name} ${t("mcp.embed.configSaved")}`,
      });
    } catch {
      toast({
        title: t("error.server"),
        description: t("mcp.embed.saveError"),
        variant: "destructive",
      });
    } finally {
      setSavingId(null);
    }
  };

  const filteredMCPs = embedMCPs.filter(
    (mcp) =>
      mcp.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      mcp.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      mcp.tags?.some((tag) =>
        tag.toLowerCase().includes(searchTerm.toLowerCase())
      )
  );

  if (loading) {
    return <div className="flex justify-center p-8">{t("common.loading")}</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <Input
          className="max-w-xs"
          placeholder={t("mcp.list.search")}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
        <Button onClick={fetchEmbedMCPs}>{t("mcp.refresh")}</Button>
      </div>

      {filteredMCPs.length === 0 ? (
        <div className="text-center p-8">
          {t("mcp.embed.noEmbeddedServers")}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredMCPs.map((mcp) => (
            <Card key={mcp.id} className="overflow-hidden">
              <CardHeader>
                <div className="flex justify-between items-start">
                  <div>
                    <CardTitle>{mcp.name}</CardTitle>
                    <div className="text-sm text-muted-foreground">
                      {mcp.id}
                    </div>
                  </div>
                  <div className="flex items-center space-x-2">
                    <Label htmlFor={`enable-${mcp.id}`} className="text-sm">
                      {mcp.enabled ? t("mcp.enabled") : t("mcp.disabled")}
                    </Label>
                    <Switch
                      id={`enable-${mcp.id}`}
                      checked={mcp.enabled}
                      onCheckedChange={() =>
                        handleStatusToggle(mcp.id, mcp.enabled)
                      }
                    />
                  </div>
                </div>
                <div className="flex flex-wrap gap-1 mt-1">
                  {mcp.tags?.map((tag) => (
                    <Badge key={tag} variant="outline">
                      {tag}
                    </Badge>
                  ))}
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                {mcp.readme && (
                  <div className="p-3 bg-muted rounded-md max-h-32 overflow-y-auto text-sm">
                    <pre className="whitespace-pre-wrap">{mcp.readme}</pre>
                  </div>
                )}

                <div className="space-y-3">
                  <h3 className="text-sm font-medium">
                    {t("mcp.config.title")}
                  </h3>
                  {Object.entries(mcp.config_templates).map(
                    ([key, template]) => (
                      <div key={key} className="space-y-1">
                        <Label
                          htmlFor={`${mcp.id}-${key}`}
                          className="flex items-center"
                        >
                          {template.name}
                          {template.required && (
                            <span className="text-red-500 ml-1">*</span>
                          )}
                        </Label>
                        <Input
                          id={`${mcp.id}-${key}`}
                          placeholder={template.example}
                          value={configValues[mcp.id]?.[key] || ""}
                          onChange={(e) =>
                            handleInputChange(mcp.id, key, e.target.value)
                          }
                        />
                        <p className="text-xs text-muted-foreground">
                          {template.description}
                        </p>
                      </div>
                    )
                  )}
                </div>

                <Button
                  className="w-full"
                  onClick={() => handleSave(mcp)}
                  disabled={savingId === mcp.id}
                >
                  {savingId === mcp.id
                    ? t("model.dialog.submitting")
                    : t("mcp.config.submit")}
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
};

export default EmbedMCPComponent;
