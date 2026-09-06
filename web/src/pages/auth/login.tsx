import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ArrowRight, Github, FileText, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { loginSchema, type LoginFormValues } from '@/validation/auth'
import { useLoginMutation } from '@/feature/auth/hooks'
import { LanguageSelector } from '@/components/common/LanguageSelector'
import { ThemeToggle } from '@/components/common/ThemeToggle'

export default function LoginPage() {
    const { t } = useTranslation()
    const loginMutation = useLoginMutation()
    const form = useForm<LoginFormValues>({ resolver: zodResolver(loginSchema), defaultValues: { token: '' } })
    return (
        <div className="flex min-h-dvh flex-col bg-card">
            <header className="flex h-16 shrink-0 items-center justify-between gap-3 border-b px-4 sm:px-8">
                <a href="https://github.com/labring/aiproxy" className="flex items-center gap-2 text-sm font-semibold"><img src="/logo.svg" alt="" className="size-7" />AI Proxy</a>
                <div className="flex items-center gap-2">
                    <a href="https://github.com/labring/aiproxy" target="_blank" rel="noopener noreferrer" title="GitHub" aria-label="GitHub" className="flex size-9 items-center justify-center rounded-md hover:bg-muted"><Github className="size-4" /></a>
                    <a href="/swagger/index.html" target="_blank" rel="noopener noreferrer" title="API Documentation" aria-label="API Documentation" className="flex size-9 items-center justify-center rounded-md hover:bg-muted"><FileText className="size-4" /></a>
                    <ThemeToggle /><LanguageSelector />
                </div>
            </header>
            <main className="flex flex-1 items-center justify-center px-6 py-12">
                <div className="w-full max-w-sm">
                    <img src="/logo.svg" alt="" className="mb-6 size-12" />
                    <h1 className="text-2xl font-semibold">AI Proxy</h1>
                    <h2 className="mt-2 text-lg font-medium">{t('auth.login.title')}</h2>
                    <p className="mt-2 text-sm text-muted-foreground">{t('auth.login.description')}</p>
                    <Form {...form}>
                        <form onSubmit={form.handleSubmit(values => loginMutation.mutate(values.token))} className="mt-8 space-y-5">
                            <FormField control={form.control} name="token" render={({ field }) => <FormItem>
                                <FormLabel>{t('auth.login.token')}</FormLabel>
                                <FormControl><Input {...field} type="password" autoComplete="current-password" className="h-11" placeholder={t('auth.login.tokenPlaceholder')} disabled={loginMutation.isPending} /></FormControl>
                                <FormMessage />
                            </FormItem>} />
                            <Button type="submit" className="h-11 w-full" disabled={loginMutation.isPending}>
                                {loginMutation.isPending ? <Loader2 className="size-4 animate-spin" /> : <ArrowRight className="size-4" />}
                                {t(loginMutation.isPending ? 'auth.login.loading' : 'auth.login.submit')}
                            </Button>
                        </form>
                    </Form>
                    <p className="mt-5 text-xs text-muted-foreground">{t('auth.login.keepSafe')}</p>
                </div>
            </main>
            <footer className="px-6 py-5 text-center text-xs text-muted-foreground">Sealos {new Date().getFullYear()} · {t('auth.login.allRightsReserved')}</footer>
        </div>
    )
}
