// src/feature/model/components/ModelTable.tsx
import { useState, useMemo, useRef } from "react";
import { useModels, useModelSets } from "../hooks";
import { useChannelInfoMap, useChannelTypeMetas } from "@/feature/channel/hooks";
import { useRuntimeMetrics } from "@/feature/monitor/runtime-hooks";
import { ModelConfig, ModelSaveRequest } from "@/types/model";
import { PriceDisplay } from "@/components/price/PriceDisplay";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  MoreHorizontal,
  Plus,
  Trash2,
  RefreshCcw,
  Pencil,
  FileText,
  Search,
  Download,
  Upload,
  Copy,
  Sparkles,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ModelDialog } from "./ModelDialog";
import { BuiltinModelsDialog } from "./BuiltinModelsDialog";
import { DeleteModelDialog } from "./DeleteModelDialog";
import { useTranslation } from "react-i18next";
import { DataTable } from "@/components/table/motion-data-table";
import { ColumnDef } from "@tanstack/react-table";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
} from "@tanstack/react-table";
import { AdvancedErrorDisplay } from "@/components/common/error/errorDisplay";
import { AnimatedButton } from "@/components/ui/animation/components/animated-button";
import { AnimatedIcon } from "@/components/ui/animation/components/animated-icon";
import ApiDocDrawer from "./api-doc/ApiDoc";
import { Badge } from "@/components/ui/badge";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ChannelDialog } from "@/feature/channel/components/ChannelDialog";
import { Channel } from "@/types/channel";
import { channelApi } from "@/api/channel";
import { modelApi } from "@/api/model";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { openResourceDialog, showDeletedResourceToast } from "@/utils/resource-dialog";
import { getChannelModelMetric } from "@/utils/runtime-metrics";
import { writeTextToClipboard } from "@/lib/clipboard";
import { ChannelLabel } from "@/components/common/ChannelLabel";

export function ModelTable() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);

  // State management
  const [modelDialogOpen, setModelDialogOpen] = useState(false);
  const [builtinModelsDialogOpen, setBuiltinModelsDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [selectedModelId, setSelectedModelId] = useState<string | null>(null);
  const [dialogMode, setDialogMode] = useState<"create" | "update">("create");
  const [selectedModel, setSelectedModel] = useState<ModelConfig | null>(null);
  const [baseModelConfig, setBaseModelConfig] = useState<ModelConfig | null>(null);
  const [preserveModelNameOnCreate, setPreserveModelNameOnCreate] = useState(false);
  const [isRefreshAnimating, setIsRefreshAnimating] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [ownerFilter, setOwnerFilter] = useState('');

  // API Doc drawer state
  const [apiDocOpen, setApiDocOpen] = useState(false);

  // Channel edit dialog state
  const [channelDialogOpen, setChannelDialogOpen] = useState(false);
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null);

  // Get models list
  const { data: models, isLoading, error, isError, refetch } = useModels();

  // Get model sets data
  const { data: modelSets, isLoading: isLoadingModelSets } = useModelSets();
  const modelSetChannelIds = useMemo(() => {
    const ids = new Set<number>();
    Object.values(modelSets ?? {}).forEach((sets) => {
      Object.values(sets).forEach((channels) => channels.forEach((channel) => ids.add(channel.id)));
    });
    return [...ids];
  }, [modelSets]);
  const { data: channelInfoMap = {} } = useChannelInfoMap(modelSetChannelIds, !isLoadingModelSets);
  const { data: runtimeMetrics } = useRuntimeMetrics();

  // Get channel type metadata
  const { data: channelTypeMetas, isLoading: isLoadingTypeMetas } = useChannelTypeMetas();

  // Sort and filter models
  const sortedModels = useMemo(() => {
    if (!models) return [];
    let filtered = models;
    if (searchKeyword) {
      const keyword = searchKeyword.toLowerCase();
      filtered = filtered.filter(m =>
        m.model.toLowerCase().includes(keyword) || (m.owner || '').toLowerCase().includes(keyword)
      );
    }
    if (ownerFilter === '__all__') {
      // no-op
    } else if (ownerFilter === '__empty__') {
      filtered = filtered.filter((m) => !m.owner);
    } else if (ownerFilter) {
      filtered = filtered.filter((m) => (m.owner || '') === ownerFilter);
    }
    return [...filtered].sort((a, b) => {
      if (a.type === b.type) {
        return a.model.localeCompare(b.model);
      }
      return a.type - b.type;
    });
  }, [models, searchKeyword, ownerFilter]);

  const ownerOptions = useMemo(() => {
    if (!models) {
      return [];
    }

    const ownerSet = new Set<string>();
    let hasEmptyOwner = false;

    for (const model of models) {
      if (model.owner) {
        ownerSet.add(model.owner);
      } else {
        hasEmptyOwner = true;
      }
    }

    const options = [...ownerSet]
      .sort((a, b) => a.localeCompare(b))
      .map((owner) => ({ value: owner, label: owner }));

    if (hasEmptyOwner) {
      options.push({ value: '__empty__', label: t("model.emptyOwner") });
    }

    return options;
  }, [models, t]);

  // Get channel type name by type ID
  const getChannelTypeName = (typeId: number): string => {
    if (!channelTypeMetas) return `Type: ${typeId}`;

    const typeKey = String(typeId);
    return channelTypeMetas[typeKey]?.name || `Type: ${typeId}`;
  };

  const toModelSaveRequest = (model: ModelConfig): ModelSaveRequest => {
    const rest = { ...model };
    delete rest.created_at;
    delete rest.updated_at;
    return rest;
  };

  const getConfigSummary = (config?: ModelConfig["config"]) => {
    if (!config) return [];

    const summary: string[] = [];
    if (config.max_context_tokens) {
      summary.push(`ctx ${config.max_context_tokens.toLocaleString()}`);
    }
    if (config.max_input_tokens) {
      summary.push(`in ${config.max_input_tokens.toLocaleString()}`);
    }
    if (config.max_output_tokens) {
      summary.push(`out ${config.max_output_tokens.toLocaleString()}`);
    }
    if (config.tool_choice) {
      summary.push("tool");
    }
    if (config.vision) {
      summary.push("vision");
    }
    if (config.coder) {
      summary.push("coder");
    }
    if (config.limited_time_free) {
      summary.push("free");
    }
    if (config.support_formats?.length) {
      summary.push(`formats ${config.support_formats.length}`);
    }
    if (config.support_voices?.length) {
      summary.push(`voices ${config.support_voices.length}`);
    }
    return summary;
  };

  const formatPercent = (value?: number) => `${((value || 0) * 100).toFixed(1)}%`;

  // Create table columns

  const columns: ColumnDef<ModelConfig>[] = useMemo(() => [
    {
      accessorKey: "model",
      header: () => (
        <div className="font-medium py-3.5">{t("model.modelName")}</div>
      ),
      cell: ({ row }) => (
        <div
          className="font-medium cursor-pointer hover:text-primary transition-colors"
          onClick={() => {
            writeTextToClipboard(row.original.model).then(() => {
              toast.success(t("common.copied"));
            }).catch(() => {
              toast.error(t("common.copyFailed"));
            });
          }}
        >
          {row.original.model}
        </div>
      ),
    },
    {
      accessorKey: "type",
      header: () => (
        <div className="font-medium py-3.5">{t("model.modelType")}</div>
      ),
      cell: ({ row }) => (
        <div
          className="font-medium cursor-pointer hover:text-primary transition-colors"
          onClick={() => openUpdateDialog(row.original)}
        >
          {/* @ts-expect-error 动态翻译键 */}
          {t(`modeType.${row.original.type}`)}
        </div>
      ),
    },
    {
      accessorKey: "owner",
      header: () => (
        <div className="font-medium py-3.5">{t("model.owner")}</div>
      ),
      cell: ({ row }) => (
        <div
          className="font-medium cursor-pointer hover:text-primary transition-colors"
          onClick={() => openUpdateDialog(row.original)}
        >
          {row.original.owner || (
            <span className="text-muted-foreground">{t("model.emptyOwner")}</span>
          )}
        </div>
      ),
    },
    {
      id: "runtime",
      header: () => (
        <div className="font-medium py-3.5">{t("common.runtime")}</div>
      ),
      cell: ({ row }) => {
        const metric = runtimeMetrics?.models?.[row.original.model];
        if (!metric) {
          return <div className="text-muted-foreground text-sm">-</div>;
        }

        return (
          <div className="flex flex-wrap gap-1">
            <Badge variant="outline" className="text-xs">RPM {metric.rpm.toLocaleString()}</Badge>
            <Badge variant="outline" className="text-xs">TPM {metric.tpm.toLocaleString()}</Badge>
            <Badge variant="outline" className="text-xs">ERR {formatPercent(metric.error_rate)}</Badge>
            {metric.banned_channels > 0 && (
              <Badge variant="destructive" className="text-xs">
                BAN {metric.banned_channels}
              </Badge>
            )}
          </div>
        );
      },
    },
    {
      accessorKey: "sets",
      header: () => (
        <div className="font-medium py-3.5">{t("model.accessibleSets")}</div>
      ),
      cell: ({ row }) => {
        const modelName = row.original.model;
        const modelSetData = modelSets?.[modelName];

        if (isLoadingModelSets || isLoadingTypeMetas) {
          return (
            <div className="text-muted-foreground text-sm">
              {t("model.loading")}
            </div>
          );
        }

        if (!modelSetData || Object.keys(modelSetData).length === 0) {
          return (
            <div className="text-muted-foreground text-sm">
              {t("model.noChannel")}
            </div>
          );
        }

        return (
          <div className="flex flex-wrap gap-1">
            {Object.entries(modelSetData).map(([setName, channels]) => (
              <Popover key={setName}>
                <PopoverTrigger asChild>
                  <Badge
                    variant="outline"
                    className="text-xs bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-900/20 dark:text-blue-400 dark:border-blue-800 cursor-pointer hover:bg-blue-100 dark:hover:bg-blue-900/30 transition-colors"
                  >
                    <span>{setName || "default"}</span>
                    <span className="ml-1 text-[11px] opacity-80">
                      {t("model.channelCount", { count: channels.length })}
                    </span>
                  </Badge>
                </PopoverTrigger>
                <PopoverContent
                  className="w-auto max-w-[calc(100vw-2rem)] p-3"
                  align="start"
                  sticky="always"
                  collisionPadding={16}
                >
                  <div className="space-y-2">
                    <h4 className="font-medium">
                      {t("model.availableChannels")} ({t("model.channelCount", { count: channels.length })})
                    </h4>
                    <div className="flex flex-col gap-1">
                      {[...channels].sort((a, b) => (b.weight ?? 0) - (a.weight ?? 0)).map((channel) => (
                        <div
                          key={channel.id}
                          className="flex flex-wrap items-center gap-2 cursor-pointer hover:bg-muted/50 rounded px-1 py-0.5 transition-colors"
                          onClick={() => {
                            openResourceDialog({
                              fetcher: () => channelApi.getChannel(channel.id),
                              onSuccess: (fullChannel) => {
                              setSelectedChannel(fullChannel);
                              setChannelDialogOpen(true);
                              },
                              onNotFound: () => {
                                showDeletedResourceToast(t("channel.deleted"));
                              },
                              onError: () => {
                                showDeletedResourceToast(t("channel.fetchFailed"));
                              },
                            });
                          }}
                        >
                          <Badge variant="secondary" className="max-w-full text-xs">
                            <ChannelLabel id={channel.id} info={channelInfoMap[channel.id] ?? channel} compact />
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            ID: {channel.id}, {getChannelTypeName(channel.type)}, {t("channel.priority")}: {channel.priority}
                          </span>
                          {(() => {
                            const pair = getChannelModelMetric(runtimeMetrics, channel.id, modelName);
                            if (!pair) return null;
                            return (
                              <div className="flex items-center gap-1 ml-auto">
                                <Badge variant="outline" className="text-[11px]">RPM {pair.rpm}</Badge>
                                <Badge variant="outline" className="text-[11px]">TPM {pair.tpm}</Badge>
                                <Badge variant="outline" className="text-[11px]">ERR {formatPercent(pair.error_rate)}</Badge>
                                {pair.banned && (
                                  <Badge variant="destructive" className="text-[11px]">{t("channel.temporarilyExcluded")}</Badge>
                                )}
                              </div>
                            );
                          })()}
                          <Badge variant="outline" className="text-xs ml-auto">
                            {(channel.weight ?? 0).toFixed(1)}%
                          </Badge>
                        </div>
                      ))}
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            ))}
          </div>
        );
      },
    },
    {
      accessorKey: "plugin",
      header: () => (
        <div className="font-medium py-3.5">{t("model.pluginInfo")}</div>
      ),
      cell: ({ row }) => {
        const plugin = row.original.plugin;
        if (!plugin) {
          return (
            <div
              className="text-muted-foreground text-sm cursor-pointer hover:text-primary transition-colors"
              onClick={() => openUpdateDialog(row.original)}
            >
              {t("model.noPluginConfigured")}
            </div>
          );
        }

        const enabledPlugins = [];

        if (plugin.cache?.enable) {
          enabledPlugins.push(t("model.cachePlugin"));
        }

        if (plugin.cachefollow?.enable) {
          enabledPlugins.push(t("model.cacheFollowPlugin"));
        }

        if (plugin["web-search"]?.enable) {
          enabledPlugins.push(t("model.webSearchPlugin"));
        }

        if (plugin["think-split"]?.enable) {
          enabledPlugins.push(t("model.thinkSplitPlugin"));
        }

        if (plugin["stream-fake"]?.enable) {
          enabledPlugins.push(t("model.streamFakePlugin"));
        }

        if (enabledPlugins.length === 0) {
          return (
            <div
              className="text-muted-foreground text-sm cursor-pointer hover:text-primary transition-colors"
              onClick={() => openUpdateDialog(row.original)}
            >
              {t("model.noPluginConfigured")}
            </div>
          );
        }

        return (
          <button
            type="button"
            className="flex max-w-44 flex-col items-start gap-1 text-left text-xs leading-5 hover:text-primary focus-visible:outline-ring"
            title={enabledPlugins.join(', ')}
            onClick={() => openUpdateDialog(row.original)}
          >
            {enabledPlugins.slice(0, 2).map((pluginName) => <span key={pluginName} className="flex items-center gap-1.5 whitespace-nowrap"><span className="size-1.5 shrink-0 rounded-full bg-emerald-500" />{pluginName}</span>)}
            {enabledPlugins.length > 2 && <span className="text-muted-foreground">+{enabledPlugins.length - 2}</span>}
          </button>
        );
      },
    },
    {
      accessorKey: "config",
      header: () => (
        <div className="font-medium py-3.5">{t("model.configInfo")}</div>
      ),
      cell: ({ row }) => {
        const config = row.original.config;
        const summary = getConfigSummary(config);

        if (!config || summary.length === 0) {
          return (
            <div
              className="text-muted-foreground text-sm cursor-pointer hover:text-primary transition-colors"
              onClick={() => openUpdateDialog(row.original)}
            >
              {t("model.noConfigConfigured")}
            </div>
          );
        }

        return (
          <button
            type="button"
            className="grid min-w-32 gap-1 text-left text-xs leading-5 hover:text-primary focus-visible:outline-ring"
            title={summary.join(', ')}
            onClick={() => openUpdateDialog(row.original)}
          >
            <span className="whitespace-nowrap font-medium">{summary[0]}</span>
            <span className="whitespace-nowrap text-muted-foreground">{summary[1]}{summary.length > 2 && <span className="ml-2">+{summary.length - 2}</span>}</span>
          </button>
        );
      },
    },
    {
      accessorKey: "price",
      header: () => (
        <div className="font-medium py-3.5">{t("model.priceColumn")}</div>
      ),
      cell: ({ row }) => <PriceDisplay price={row.original.price} />,
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={t("ui.actions")}>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => openApiDoc(row.original)}>
              <FileText className="mr-2 h-4 w-4" />
              {t("model.apiDetails")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => openUpdateDialog(row.original)}>
              <Pencil className="mr-2 h-4 w-4" />
              {t("model.edit")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => openCopyDialog(row.original)}>
              <Copy className="mr-2 h-4 w-4" />
              {t("model.copyFrom")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => openDeleteDialog(row.original.model)}
            >
              <Trash2 className="mr-2 h-4 w-4 text-red-600 dark:text-red-500" />
              {t("model.delete")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => exportSingleModel(row.original)}>
              <Download className="mr-2 h-4 w-4" />
              {t("model.export")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ], [t, modelSets, channelInfoMap, channelTypeMetas, isLoadingModelSets, isLoadingTypeMetas, runtimeMetrics]);

  // Initialize table
  const table = useReactTable({
    data: sortedModels,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    initialState: {
      sorting: [
        {
          id: "type",
          desc: false,
        },
      ],
    },
  });

  // Open create model dialog
  const openCreateDialog = () => {
    setDialogMode("create");
    setSelectedModel(null);
    setBaseModelConfig(null);
    setPreserveModelNameOnCreate(false);
    setModelDialogOpen(true);
  };

  // Open update model dialog
  const openUpdateDialog = (model: ModelConfig) => {
    setDialogMode("update");
    setSelectedModel(model);
    setBaseModelConfig(model);
    setPreserveModelNameOnCreate(false);
    setModelDialogOpen(true);
  };

  // Open copy model dialog (create mode with existing model data)
  const openCopyDialog = (model: ModelConfig) => {
    setDialogMode("create");
    setSelectedModel(model);
    setBaseModelConfig(model);
    setPreserveModelNameOnCreate(false);
    setModelDialogOpen(true);
  };

  const openCreateFromBuiltinDialog = (model: ModelConfig) => {
    setDialogMode("create");
    setSelectedModel(model);
    setBaseModelConfig(model);
    setPreserveModelNameOnCreate(true);
    setModelDialogOpen(true);
  };

  const openEditFromBuiltinDialog = (builtinModel: ModelConfig, existingModel: ModelConfig) => {
    setDialogMode("update");
    setSelectedModel(builtinModel);
    setBaseModelConfig(existingModel);
    setPreserveModelNameOnCreate(false);
    setModelDialogOpen(true);
  };

  // Open delete dialog
  const openDeleteDialog = (id: string) => {
    setSelectedModelId(id);
    setDeleteDialogOpen(true);
  };

  // Open API documentation drawer
  const openApiDoc = (model: ModelConfig) => {
    setSelectedModel(model);
    setApiDocOpen(true);
  };

  // Refresh models
  const refreshModels = () => {
    setIsRefreshAnimating(true);
    refetch();

    // Stop animation after 1 second
    setTimeout(() => {
      setIsRefreshAnimating(false);
    }, 1000);
  };

  // Export model configs to JSON file
  const exportModels = () => {
    if (!models || models.length === 0) {
      toast.error(t("model.noDataToExport"));
      return;
    }

    const exportData = models.map(toModelSaveRequest);

    const blob = new Blob([JSON.stringify(exportData, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `model_configs_${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success(t("model.exportSuccess"));
  };

  // Export single model config to JSON file
  const exportSingleModel = (model: ModelConfig) => {
    const exportData = [toModelSaveRequest(model)];

    const blob = new Blob([JSON.stringify(exportData, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `model_${model.model}_${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success(t("model.exportSuccess"));
  };

  // Import model configs from JSON file
  const importModels = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    setIsImporting(true);
    try {
      const text = await file.text();
      const configs: ModelSaveRequest[] = JSON.parse(text);

      if (!Array.isArray(configs)) {
        throw new Error(t("model.invalidFormat"));
      }

      await modelApi.saveModels(configs);
      toast.success(t("model.importSuccess", { count: configs.length }));
      queryClient.invalidateQueries({ queryKey: ["models"] });
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t("model.importFailed")
      );
    } finally {
      setIsImporting(false);
      // Reset file input
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  // Trigger file input click
  const triggerImport = () => {
    fileInputRef.current?.click();
  };

  return (
    <>
      <section className="resource-page">
        {/* Title and action buttons */}
        <div className="contents">
        <div className="resource-header">
          <h2 className="text-lg font-semibold text-foreground">
            {t("model.management")} <Badge variant="secondary" className="ml-2 rounded-full">{sortedModels.length}</Badge>
          </h2>

          <div className="resource-actions">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("common.search")}
                value={searchKeyword}
                onChange={(e) => setSearchKeyword(e.target.value)}
                className="h-9 w-full pl-8 sm:w-56"
              />
            </div>
            <div className="w-44">
              <Select value={ownerFilter} onValueChange={setOwnerFilter}>
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={t("model.ownerFilterPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all__">{t("model.allOwners")}</SelectItem>
                  {ownerOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <AnimatedButton>
              <Button
                variant="outline"
                size="sm"
                onClick={refreshModels}
                className="flex items-center gap-2 justify-center"
              >
                <AnimatedIcon
                  animationVariant="continuous-spin"
                  isAnimating={isRefreshAnimating}
                  className="h-4 w-4"
                >
                  <RefreshCcw className="h-4 w-4" />
                </AnimatedIcon>
                {t("model.refresh")}
              </Button>
            </AnimatedButton>
            <AnimatedButton>
              <Button
                variant="outline"
                size="sm"
                onClick={exportModels}
                disabled={!models || models.length === 0}
                className="flex items-center gap-1"
              >
                <Download className="h-4 w-4" />
                {t("model.export")}
              </Button>
            </AnimatedButton>
            <AnimatedButton>
              <Button
                variant="outline"
                size="sm"
                onClick={triggerImport}
                disabled={isImporting}
                className="flex items-center gap-1"
              >
                <Upload className="h-4 w-4" />
                {isImporting ? t("model.importing") : t("model.import")}
              </Button>
            </AnimatedButton>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json"
              onChange={importModels}
              className="hidden"
            />
            <AnimatedButton>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setBuiltinModelsDialogOpen(true)}
                className="flex items-center gap-1"
              >
                <Sparkles className="h-4 w-4" />
                {t("model.builtin.trigger")}
              </Button>
            </AnimatedButton>
            <AnimatedButton>
              <Button
                size="sm"
                onClick={openCreateDialog}
                className="flex items-center gap-1"
              >
                <Plus className="h-4 w-4" />
                {t("model.add")}
              </Button>
            </AnimatedButton>
          </div>
        </div>
        </div>

        {/* Table container */}
        <div className="resource-table">
          <div className="resource-table-body">
            {isError ? (
              <AdvancedErrorDisplay error={error} onRetry={refetch} />
            ) : (
              <DataTable
                table={table}
                columns={columns}
                isLoading={isLoading}
                loadingStyle="skeleton"
                fixedHeader={true}
                animatedRows={true}
                showScrollShadows={true}
              />
            )}
          </div>
        </div>
      </section>

      <BuiltinModelsDialog
        open={builtinModelsDialogOpen}
        onOpenChange={setBuiltinModelsDialogOpen}
        existingModels={models || []}
        onCreateFromBuiltin={openCreateFromBuiltinDialog}
        onEditFromBuiltin={openEditFromBuiltinDialog}
      />

      {/* Model Dialog */}
      <ModelDialog
        open={modelDialogOpen}
        onOpenChange={setModelDialogOpen}
        mode={dialogMode}
        model={selectedModel}
        baseModelConfig={baseModelConfig}
        preserveModelNameOnCreate={preserveModelNameOnCreate}
      />

      {/* Delete Model Dialog */}
      <DeleteModelDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        modelId={selectedModelId}
        onDeleted={() => setSelectedModelId(null)}
      />

      {/* API Documentation Drawer */}

      {selectedModel && (
        <ApiDocDrawer
          isOpen={apiDocOpen}
          onClose={() => setApiDocOpen(false)}
          modelConfig={selectedModel}
        />
      )}

      {/* Channel Edit Dialog */}
      <ChannelDialog
        open={channelDialogOpen}
        onOpenChange={setChannelDialogOpen}
        mode="update"
        channel={selectedChannel}
      />
    </>
  );
}
