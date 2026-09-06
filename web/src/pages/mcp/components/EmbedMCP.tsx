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
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ChevronDown, RefreshCw, Save, Settings2 } from "lucide-react";

const EmbedMCPComponent = () => {
  const [embedMCPs, setEmbedMCPs] = useState<EmbedMCP[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [configValues, setConfigValues] = useState<
    Record<string, Record<string, string>>
  >({});
  const [savingId, setSavingId] = useState<string | null>(null);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());
  const { toast } = useToast();
  const { t, i18n } = useTranslation();

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
        initialConfigValues[mcp.id] = { ...mcp.embed_config?.init };
        Object.entries(mcp.config_templates).forEach(([key, template]) => {
          initialConfigValues[mcp.id][key] = mcp.embed_config?.init?.[key] ?? template.example ?? "";
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
      mcp.name_cn?.toLowerCase().includes(searchTerm.toLowerCase()) ||
      mcp.tags?.some((tag) =>
        tag.toLowerCase().includes(searchTerm.toLowerCase())
      )
  );

  if (loading) {
    return <div className="flex justify-center p-8">{t("common.loading")}</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <Input
          className="max-w-xs"
          placeholder={t("mcp.list.search")}
          aria-label={t("mcp.list.search")}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
        <Button variant="outline" size="icon" title={t("mcp.refresh")} aria-label={t("mcp.refresh")} onClick={fetchEmbedMCPs}><RefreshCw className="size-4" /></Button>
      </div>

      {filteredMCPs.length === 0 ? (
        <div className="text-center p-8">
          {t("mcp.embed.noEmbeddedServers")}
        </div>
      ) : (
        <div className="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
          {filteredMCPs.map((mcp) => (
            <Card key={mcp.id} className="overflow-hidden">
              <CardHeader>
                <div className="flex flex-wrap justify-between items-start gap-3">
                  <div className="min-w-0 flex-1">
                    <CardTitle className="break-words text-base">{i18n.language.startsWith('zh') && mcp.name_cn ? mcp.name_cn : mcp.name}</CardTitle>
                    <div className="break-all text-xs font-mono text-muted-foreground">
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
                <Button variant="outline" className="w-full justify-between" aria-expanded={expandedIds.has(mcp.id)} onClick={() => setExpandedIds(prev => {
                  const next = new Set(prev);
                  if (next.has(mcp.id)) next.delete(mcp.id); else next.add(mcp.id);
                  return next;
                })}>
                  <span className="inline-flex items-center gap-2"><Settings2 className="size-4" />{t("mcp.config.title")}</span><ChevronDown className={`size-4 transition-transform ${expandedIds.has(mcp.id) ? 'rotate-180' : ''}`} />
                </Button>
                {expandedIds.has(mcp.id) && <div className="space-y-4 border-t pt-4">
                {mcp.readme && <details className="group"><summary className="cursor-pointer text-sm font-medium text-primary">README</summary><div className="markdown-content max-h-80 overflow-auto pt-3"><ReactMarkdown remarkPlugins={[remarkGfm]}>{i18n.language.startsWith('zh') && mcp.readme_cn ? mcp.readme_cn : mcp.readme}</ReactMarkdown></div></details>}
                <div className="space-y-3">
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
                </div>}

                <Button
                  className="w-full"
                  onClick={() => handleSave(mcp)}
                  disabled={savingId === mcp.id}
                >
                  <Save className="size-4" />
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
