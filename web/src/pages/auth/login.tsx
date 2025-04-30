import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { KeyRound } from 'lucide-react'
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"

import { loginSchema, type LoginFormValues } from "@/validation/auth"
import { useLoginMutation } from "@/feature/auth/hooks"
import { LanguageSelector } from "@/components/common/LanguageSelector"
import { ParticlesBackground } from "@/components/ui/animation/components/particles-background"
import { ThemeToggle } from "@/components/common/ThemeToggle"

export default function LoginPage() {
    const { t } = useTranslation()
    const loginMutation = useLoginMutation()

    const form = useForm<LoginFormValues>({
        resolver: zodResolver(loginSchema),
        defaultValues: {
            token: "",
        },
    })

    const onSubmit = (values: LoginFormValues) => {
        loginMutation.mutate(values.token)
    }

    return (
        <div className="flex min-h-screen flex-col items-center justify-center relative px-4 py-12 bg-gradient-to-br from-[#F8F9FF] to-[#EEF1FF] dark:from-gray-900 dark:via-gray-900 dark:to-gray-800 overflow-hidden">
            {/* 背景装饰 */}
            <div className="absolute inset-0 overflow-hidden">
                {/* 圆形渐变光效 */}
                <div className="absolute left-0 top-0 w-full h-60 bg-gradient-to-r from-[#6A6DE6]/20 to-[#8A8DF7]/20 blur-3xl transform -translate-y-20 rounded-full"></div>
                <div className="absolute right-0 bottom-0 w-full h-60 bg-gradient-to-l from-[#6A6DE6]/20 to-[#8A8DF7]/20 blur-3xl transform translate-y-20 rounded-full"></div>

                {/* 光晕效果 */}
                <div className="absolute left-1/4 top-1/4 w-32 h-32 bg-[#6A6DE6]/10 rounded-full blur-2xl"></div>
                <div className="absolute right-1/4 bottom-1/3 w-40 h-40 bg-[#8A8DF7]/15 rounded-full blur-3xl"></div>

                {/* 方块颗粒动画背景 */}
                <ParticlesBackground
                    particleColor="rgba(106, 109, 230, 0.08)"
                    particleSize={6}
                    particleCount={40}
                    speed={0.3}
                />
                <ParticlesBackground
                    particleColor="rgba(138, 141, 247, 0.1)"
                    particleSize={8}
                    particleCount={25}
                    speed={0.2}
                />
            </div>

            {/* Language Selector and Theme Toggle */}
            <div className="absolute top-4 right-4 z-10 flex items-center gap-4">
                <ThemeToggle />
                <LanguageSelector variant="minimal" />
            </div>

            <div className="w-full max-w-md relative z-10">
                <Card className="overflow-hidden border-0 shadow-2xl backdrop-blur-sm bg-white/90 dark:bg-gray-900/90 rounded-xl">
                    <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[#6A6DE6] to-[#8A8DF7]" />

                    <CardHeader className="space-y-5 pb-2 pt-8 flex flex-col items-center">
                        <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-[#6A6DE6] to-[#8A8DF7] p-4 mb-2 shadow-lg flex items-center justify-center relative overflow-hidden transition-all duration-300 hover:shadow-xl">
                            <div className="w-8 h-8 bg-white rounded-md flex items-center justify-center">
                                <img src="/logo.svg" alt="Logo" className="w-6 h-6" />
                            </div>
                        </div>
                        <div className="text-center">
                            <CardTitle className="text-2xl font-bold">{t("auth.login.title")}</CardTitle>
                            <CardDescription className="text-gray-500 dark:text-gray-400 mt-1">
                                {t("auth.login.description")}
                            </CardDescription>
                        </div>
                    </CardHeader>

                    <CardContent className="px-8 pb-6">
                        <Form {...form}>
                            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
                                <FormField
                                    control={form.control}
                                    name="token"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className="text-sm font-medium">{t("auth.login.token")}</FormLabel>
                                            <FormControl>
                                                <div className="relative">
                                                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                                        <KeyRound className="h-5 w-5 text-gray-400" />
                                                    </div>
                                                    <Input
                                                        {...field}
                                                        placeholder={t("auth.login.tokenPlaceholder")}
                                                        type="password"
                                                        className="h-11 pl-10 border-gray-200 bg-gray-50/50 focus:border-[#6A6DE6] focus:ring-[#6A6DE6] dark:border-gray-700 dark:bg-gray-800/50 rounded-lg"
                                                        disabled={loginMutation.isPending}
                                                    />
                                                </div>
                                            </FormControl>
                                            <FormMessage className="text-xs font-medium text-red-500" />
                                        </FormItem>
                                    )}
                                />
                                <Button
                                    type="submit"
                                    className="w-full h-11 bg-gradient-to-r from-[#6A6DE6] to-[#8A8DF7] hover:opacity-90 text-white transition-all duration-200 shadow-md hover:shadow-lg rounded-lg font-medium"
                                    disabled={loginMutation.isPending}
                                >
                                    {loginMutation.isPending ? (
                                        <div className="flex items-center justify-center">
                                            <svg
                                                className="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
                                                xmlns="http://www.w3.org/2000/svg"
                                                fill="none"
                                                viewBox="0 0 24 24"
                                            >
                                                <circle
                                                    className="opacity-25"
                                                    cx="12"
                                                    cy="12"
                                                    r="10"
                                                    stroke="currentColor"
                                                    strokeWidth="4"
                                                ></circle>
                                                <path
                                                    className="opacity-75"
                                                    fill="currentColor"
                                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                                ></path>
                                            </svg>
                                            {t("auth.login.loading")}
                                        </div>
                                    ) : (
                                        <div className="flex items-center justify-center">
                                            <KeyRound className="h-4 w-4 mr-2" />
                                            {t("auth.login.submit")}
                                        </div>
                                    )}
                                </Button>
                            </form>
                        </Form>
                    </CardContent>

                    <CardFooter className="border-t border-gray-100 dark:border-gray-800 bg-gray-50/70 dark:bg-gray-900/70 px-6 py-4">
                        <p className="w-full text-center text-sm text-gray-500 dark:text-gray-400">{t("auth.login.keepSafe")}</p>
                    </CardFooter>
                </Card>

                <div className="mt-8 text-center">
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                        © {new Date().getFullYear()} Sealos. {t("auth.login.allRightsReserved")}
                    </p>
                </div>
            </div>
        </div>
    )
}