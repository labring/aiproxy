// src/feature/model/components/ModelTable.tsx
import { useState, useMemo } from "react";
import { useModels, useModelSets } from "../hooks";
import { useChannelTypeMetas } from "@/feature/channel/hooks";
import { ModelConfig } from "@/types/model";
import { PriceDisplay } from "@/components/price/PriceDisplay";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  MoreHorizontal,
  Plus,
  Trash2,
  RefreshCcw,
  Pencil,
  FileText,
  Search,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Card } from "@/components/ui/card";
import { ModelDialog } from "./ModelDialog";
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
import { toast } from "sonner";

export function ModelTable() {
  const { t } = useTranslation();

  // State management
  const [modelDialogOpen, setModelDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [selectedModelId, setSelectedModelId] = useState<string | null>(null);
  const [dialogMode, setDialogMode] = useState<"create" | "update">("create");
  const [selectedModel, setSelectedModel] = useState<ModelConfig | null>(null);
  const [isRefreshAnimating, setIsRefreshAnimating] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');

  // API Doc drawer state
  const [apiDocOpen, setApiDocOpen] = useState(false);

  // Channel edit dialog state
  const [channelDialogOpen, setChannelDialogOpen] = useState(false);
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null);

  // Get models list
  const { data: models, isLoading, error, isError, refetch } = useModels();

  // Get model sets data
  const { data: modelSets, isLoading: isLoadingModelSets } = useModelSets();

  // Get channel type metadata
  const { data: channelTypeMetas, isLoading: isLoadingTypeMetas } = useChannelTypeMetas();

  // Sort and filter models
  const sortedModels = useMemo(() => {
    if (!models) return [];
    let filtered = models;
    if (searchKeyword) {
      const keyword = searchKeyword.toLowerCase();
      filtered = models.filter(m => m.model.toLowerCase().includes(keyword));
    }
    return [...filtered].sort((a, b) => {
      if (a.type === b.type) {
        return a.model.localeCompare(b.model);
      }
      return a.type - b.type;
    });
  }, [models, searchKeyword]);

  // Get channel type name by type ID
  const getChannelTypeName = (typeId: number): string => {
    if (!channelTypeMetas) return `Type: ${typeId}`;
    
    const typeKey = String(typeId);
    return channelTypeMetas[typeKey]?.name || `Type: ${typeId}`;
  };

  // Create table columns
  // eslint-disable-next-line react-hooks/exhaustive-deps
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
            navigator.clipboard.writeText(row.original.model).then(() => {
              toast.success(t("common.copied"));
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
                    {setName}
                  </Badge>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-3" align="start">
                  <div className="space-y-2">
                    <h4 className="font-medium">
                      {t("model.availableChannels")}
                    </h4>
                    <div className="flex flex-col gap-1">
                      {[...channels].sort((a, b) => (b.weight ?? 0) - (a.weight ?? 0)).map((channel) => (
                        <div
                          key={channel.id}
                          className="flex items-center gap-2 cursor-pointer hover:bg-muted/50 rounded px-1 py-0.5 transition-colors"
                          onClick={async () => {
                            try {
                              const fullChannel = await channelApi.getChannel(channel.id);
                              setSelectedChannel(fullChannel);
                              setChannelDialogOpen(true);
                            } catch {
                              toast.error(t("channel.fetchFailed"));
                            }
                          }}
                        >
                          <Badge variant="secondary" className="text-xs">
                            {channel.name}
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            ID: {channel.id}, {getChannelTypeName(channel.type)}, {t("channel.priority")}: {channel.priority}
                          </span>
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
          <div
            className="flex flex-wrap gap-1 cursor-pointer"
            onClick={() => openUpdateDialog(row.original)}
          >
            {enabledPlugins.map((pluginName) => (
              <Badge
                key={pluginName}
                variant="outline"
                className="text-xs bg-green-50 text-green-700 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800 hover:bg-green-100 dark:hover:bg-green-900/30 transition-colors"
              >
                {pluginName}
              </Badge>
            ))}
          </div>
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
            <Button variant="ghost" size="icon">
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
            <DropdownMenuItem
              onClick={() => openDeleteDialog(row.original.model)}
            >
              <Trash2 className="mr-2 h-4 w-4 text-red-600 dark:text-red-500" />
              {t("model.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ], [t, modelSets, channelTypeMetas, isLoadingModelSets, isLoadingTypeMetas]);

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
    setModelDialogOpen(true);
  };

  // Open update model dialog
  const openUpdateDialog = (model: ModelConfig) => {
    setDialogMode("update");
    setSelectedModel(model);
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

  return (
    <>
      <Card className="border-none shadow-none p-6 flex flex-col h-full">
        {/* Title and action buttons */}
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold text-primary">
            {t("model.management")}
          </h2>
          <div className="flex gap-2">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("common.search")}
                value={searchKeyword}
                onChange={(e) => setSearchKeyword(e.target.value)}
                className="h-9 w-48 pl-8"
              />
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

        {/* Table container */}
        <div className="flex-1 overflow-hidden flex flex-col">
          <div className="overflow-auto h-full">
            {isError ? (
              <AdvancedErrorDisplay error={error} onRetry={refetch} />
            ) : (
              <DataTable
                table={table}
                columns={columns}
                isLoading={isLoading || isLoadingModelSets || isLoadingTypeMetas}
                loadingStyle="skeleton"
                fixedHeader={true}
                animatedRows={true}
                showScrollShadows={true}
              />
            )}
          </div>
        </div>
      </Card>

      {/* Model Dialog */}
      <ModelDialog
        open={modelDialogOpen}
        onOpenChange={setModelDialogOpen}
        mode={dialogMode}
        model={selectedModel}
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
