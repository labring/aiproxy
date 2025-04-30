import { StrictMode, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import './index.css'
import './i18n'
import App from './App.tsx'
import { ErrorBoundary } from './handler/ErrorBoundary'
import { ENV } from './utils/env.ts'
import { Toaster } from '@/components/ui/sonner'
import { ConstantCategory } from './constant/index.ts'
import { getConstant } from './constant/index.ts'
import { I18nextProvider } from 'react-i18next'
import i18n from './i18n'
import { LoadingFallback } from './components/common/LoadingFallBack'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: getConstant(ConstantCategory.FEATURE, 'QUERY_STALE_TIME', 5 * 60 * 1000),
      retry: getConstant(ConstantCategory.FEATURE, 'DEFAULT_QUERY_RETRY', 1),
      refetchOnWindowFocus: false
    }
  }
})



createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <Suspense fallback={<LoadingFallback />}>
            <App />
            <Toaster />
          </Suspense>
        </I18nextProvider>
        {ENV.isDevelopment && <ReactQueryDevtools />}
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
)
