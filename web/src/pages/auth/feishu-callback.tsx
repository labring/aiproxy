import { useEffect, useRef, useState } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { useTranslation } from "react-i18next"
import { Loader2, AlertCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { enterpriseApi } from "@/api/enterprise"
import useAuthStore from "@/store/auth"
import { ROUTES } from "@/routes/constants"
import { ParticlesBackground } from "@/components/ui/animation/components/particles-background"

export default function FeishuCallbackPage() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [searchParams] = useSearchParams()
    const { loginWithFeishu } = useAuthStore()
    const [error, setError] = useState<string | null>(null)
    const processedRef = useRef(false)

    useEffect(() => {
        if (processedRef.current) return
        processedRef.current = true

        // Case 0: Backend redirected with error (e.g., unauthorized tenant)
        const errorParam = searchParams.get("error")
        if (errorParam) {
            let message = searchParams.get("message") || t("auth.feishuCallback.error")
            const tenantId = searchParams.get("tenant_id")
            if (tenantId && errorParam === "unauthorized_tenant") {
                message += `\n\nTenant ID: ${tenantId}`
            }
            setError(message)
            return
        }

        // Case 1: Backend redirected here with token_key directly (browser OAuth flow)
        const tokenKey = searchParams.get("token_key")
        if (tokenKey) {
            const user = {
                name: searchParams.get("name") || "",
                avatar: searchParams.get("avatar") || "",
                openId: searchParams.get("open_id") || "",
            }
            loginWithFeishu(tokenKey, user)
            navigate(ROUTES.ENTERPRISE, { replace: true })
            return
        }

        // Case 2: Frontend-initiated flow with authorization code
        const code = searchParams.get("code")
        if (!code) {
            setError(t("auth.feishuCallback.noCode"))
            return
        }

        const exchange = async () => {
            try {
                const resp = await enterpriseApi.feishuCallback(code)
                const user = enterpriseApi.toEnterpriseUser(resp)
                loginWithFeishu(resp.token_key, user)
                navigate(ROUTES.ENTERPRISE, { replace: true })
            } catch (err) {
                const message = err instanceof Error ? err.message : String(err)
                setError(message)
            }
        }

        exchange()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    const handleRetry = () => {
        window.location.href = enterpriseApi.feishuLoginUrl()
    }

    return (
        <div className="flex min-h-screen flex-col items-center justify-center relative px-4 py-12 bg-gradient-to-br from-[#F8F9FF] to-[#EEF1FF] dark:from-gray-900 dark:via-gray-900 dark:to-gray-800 overflow-hidden">
            <div className="absolute inset-0 overflow-hidden">
                <ParticlesBackground
                    particleColor="rgba(106, 109, 230, 0.08)"
                    particleSize={6}
                    particleCount={30}
                    speed={0.3}
                />
            </div>

            <Card className="w-full max-w-md relative z-10 border-0 shadow-2xl backdrop-blur-sm bg-white/90 dark:bg-gray-900/90 rounded-xl">
                <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[#6A6DE6] to-[#8A8DF7]" />
                <CardHeader className="text-center pt-8">
                    <CardTitle className="text-xl font-bold">
                        {error ? t("auth.feishuCallback.error") : t("auth.feishuCallback.processing")}
                    </CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col items-center gap-6 pb-8">
                    {error ? (
                        <>
                            <div className="flex items-center gap-2 text-red-500">
                                <AlertCircle className="w-5 h-5" />
                                <span className="text-sm">{error}</span>
                            </div>
                            <Button
                                onClick={handleRetry}
                                className="bg-gradient-to-r from-[#6A6DE6] to-[#8A8DF7] text-white hover:opacity-90"
                            >
                                {t("auth.feishuCallback.retry")}
                            </Button>
                        </>
                    ) : (
                        <Loader2 className="w-8 h-8 animate-spin text-[#6A6DE6]" />
                    )}
                </CardContent>
            </Card>
        </div>
    )
}
