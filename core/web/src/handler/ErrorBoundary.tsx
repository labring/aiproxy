// src/components/error-boundary.tsx
import { Component, ReactNode } from 'react'
import { isRouteErrorResponse, useRouteError } from 'react-router'
import { AlertCircle, Home, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { motion } from 'motion/react'

interface Props {
    children?: ReactNode
}

interface State {
    hasError: boolean
    error?: Error
}

export class ErrorBoundary extends Component<Props, State> {
    constructor(props: Props) {
        super(props)
        this.state = { hasError: false }
    }

    static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error }
    }

    render() {
        if (this.state.hasError) {
            return <ErrorDisplay error={this.state.error} />
        }

        return this.props.children
    }
}

interface ErrorDisplayProps {
    error?: Error
}

export function ErrorDisplay({ error }: ErrorDisplayProps) {
    const { t } = useTranslation()

    // 如果在路由中使用，尝试获取路由错误
    try {
        const routeError = useRouteError()
        if (isRouteErrorResponse(routeError)) {
            return (
                <div className="flex min-h-screen w-full flex-col items-center justify-center p-6 text-center bg-gradient-to-b from-background to-muted/30">
                    <motion.div
                        initial={{ opacity: 0, y: -20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.5 }}
                        className="w-full max-w-md p-8 rounded-lg bg-card shadow-lg border border-border"
                    >
                        <motion.div
                            initial={{ scale: 0.8 }}
                            animate={{ scale: 1 }}
                            transition={{ delay: 0.2, type: "spring" }}
                            className="mx-auto bg-red-100 p-3 rounded-full w-16 h-16 flex items-center justify-center mb-6"
                        >
                            <AlertCircle className="h-10 w-10 text-red-500" />
                        </motion.div>
                        <h1 className="text-3xl font-bold mb-3">{t("error.code", "错误")} {routeError.status}</h1>
                        <p className="mb-4 text-muted-foreground text-lg">{routeError.statusText}</p>
                        <p className="text-sm text-muted-foreground mb-6">{routeError.data?.message || t("error.unexpected", "发生了意外错误")}</p>

                        <div className="flex flex-col sm:flex-row gap-3 justify-center">
                            <Button
                                onClick={() => window.location.reload()}
                                variant="outline"
                                className="gap-2"
                            >
                                <RefreshCw className="h-4 w-4" />
                                {t("error.refresh", "刷新页面")}
                            </Button>
                            <Button
                                onClick={() => window.location.href = '/'}
                                className="gap-2"
                            >
                                <Home className="h-4 w-4" />
                                {t("error.returnHome", "返回首页")}
                            </Button>
                        </div>
                    </motion.div>
                </div>
            )
        }

        if (routeError instanceof Error) {
            error = routeError
        }
    } catch {
        // useRouteError 只能在路由上下文中使用
    }

    return (
        <div className="flex min-h-screen w-full flex-col items-center justify-center p-6 text-center bg-gradient-to-b from-background to-muted/30">
            <motion.div
                initial={{ opacity: 0, y: -20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.5 }}
                className="w-full max-w-md p-8 rounded-lg bg-card shadow-lg border border-border"
            >
                <motion.div
                    initial={{ scale: 0.8 }}
                    animate={{ scale: 1 }}
                    transition={{ delay: 0.2, type: "spring" }}
                    className="mx-auto bg-red-100 p-3 rounded-full w-16 h-16 flex items-center justify-center mb-6"
                >
                    <AlertCircle className="h-10 w-10 text-red-500" />
                </motion.div>
                <h1 className="text-3xl font-bold mb-3">{t("error.title", "发生了错误")}</h1>
                <p className="mb-4 text-muted-foreground text-lg">{error?.message || t("error.appProblem", "应用程序遇到了意外问题")}</p>

                {error?.stack && (
                    <div className="mb-6 mx-auto max-w-md">
                        <div className="bg-muted p-3 rounded-md text-left overflow-x-auto text-xs font-mono text-muted-foreground mb-4">
                            {error.stack.split('\n').slice(0, 3).join('\n')}
                        </div>
                    </div>
                )}

                <div className="flex flex-col sm:flex-row gap-3 justify-center">
                    <Button
                        onClick={() => window.location.reload()}
                        variant="outline"
                        className="gap-2"
                    >
                        <RefreshCw className="h-4 w-4" />
                        {t("error.refresh", "刷新页面")}
                    </Button>
                    <Button
                        onClick={() => window.location.href = '/'}
                        className="gap-2"
                    >
                        <Home className="h-4 w-4" />
                        {t("error.returnHome", "返回首页")}
                    </Button>
                </div>
            </motion.div>
        </div>
    )
}

// 用作路由错误元素的简单包装器
export function RouteErrorBoundary() {
    return <ErrorDisplay />
}