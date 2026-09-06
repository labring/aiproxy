import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Check, Copy } from 'lucide-react'
import { writeTextToClipboard } from '@/lib/clipboard'

interface CopyButtonProps {
  text: string
  className?: string
}

export const CopyButton = ({ text, className }: CopyButtonProps) => {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])

  const copyToClipboard = () => {
    writeTextToClipboard(text).then(() => {
      setCopied(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 2000)
    }).catch(() => toast.error(t('common.copyFailed')))
  }

  return (
    <Button
      type="button"
      size="icon"
      variant="ghost"
      aria-label={t(copied ? 'common.copied' : 'ui.copy')}
      title={t(copied ? 'common.copied' : 'ui.copy')}
      className={className}
      onClick={copyToClipboard}
    >
      {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
    </Button>
  )
}
