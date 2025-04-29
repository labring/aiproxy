import React from 'react'
import {
    Sheet,
    SheetContent,
} from "@/components/ui/sheet"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ModelConfig } from '@/types/model'
import { useTranslation } from 'react-i18next'
import CodeBlock from './CodeHight'
import { toast } from 'sonner'
import { TFunction } from 'i18next'

interface ApiDocContent {
    title: string
    endpoint: string
    method: string
    requestExample: string
    responseExample: string
    responseFormat: string
    requestAdditionalInfo?: {
        voices?: string[]
        formats?: string[]
    }
    responseAdditionalInfo?: {
        voices?: string[]
        formats?: string[]
    }
}

interface ApiDocDrawerProps {
    isOpen: boolean
    onClose: () => void
    modelConfig: ModelConfig
}

const getApiDocContent = (
    modelConfig: ModelConfig,
    apiEndpoint: string,
    t: TFunction
): ApiDocContent => {
    switch (modelConfig.type) {
        case 1:
            return {
                title: t('modeType.1'),
                endpoint: '/chat/completions',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/chat/completions \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "messages": [
    {
      "role": "user",
      "content": "What is Sealos"
    }
  ],
  "stream": false,
  "max_tokens": 512,
  "temperature": 0.7
}'`,
                responseExample: `{
  "object": "chat.completion",
  "created": 1729672480,
  "model": "${modelConfig.model}",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Sealos is a cloud operating system based on Kubernetes, designed to provide users with a simple, efficient, and scalable cloud-native application deployment and management experience."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 18,
    "completion_tokens": 52,
    "total_tokens": 70
  }
}`
            }
        case 3:
            return {
                title: t('modeType.3'),
                endpoint: '/embeddings',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/embeddings \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "input": "Your text string goes here",
  "encoding_format": "float"
}'`,
                responseExample: `{
  "object": "list",
  "model": "${modelConfig.model}",
  "data": [
    {
      "object": "embedding",
      "embedding": [
        -0.1082494854927063,
        0.022976752370595932
        ...
      ],
      "index": 0
    }
  ],
  "usage": {
    "prompt_tokens": 4,
    "completion_tokens": 0,
    "total_tokens": 4
  }
}`
            }
        case 7:
            return {
                title: t('modeType.7'),
                endpoint: '/audio/speech',
                method: 'POST',
                responseFormat: 'binary',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/audio/speech \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "input": "The text to generate audio for",
${modelConfig?.config?.support_voices?.length
                        ? `  "voice": "${modelConfig.config.support_voices[0]}",\n`
                        : ''
                    }${modelConfig?.config?.support_formats?.length
                        ? `  "response_format": "${modelConfig.config.support_formats[0]}",\n`
                        : ''
                    }  "stream": true,
  "speed": 1
}' > audio.mp3`,
                responseExample: 'Binary audio data',
                requestAdditionalInfo: {
                    voices: modelConfig?.config?.support_voices,
                    formats: modelConfig?.config?.support_formats
                }
            }
        case 8:
            return {
                title: t('modeType.8'),
                endpoint: '/audio/transcriptions',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/audio/transcriptions \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: multipart/form-data' \\
--form model=${modelConfig.model} \\
--form 'file=@"audio.mp3"'`,
                responseExample: `{
  "text": "<string>"
}`
            }
        case 10:
            return {
                title: t('modeType.10'),
                endpoint: '/rerank',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/rerank \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "query": "Apple",
  "documents": [
    "Apple",
    "Banana",
    "Fruit",
    "Vegetable"
  ],
  "top_n": 4,
  "return_documents": false,
  "max_chunks_per_doc": 1024,
  "overlap_tokens": 80
}'`,
                responseExample: `{
  "results": [
    {
      "index": 0,
      "relevance_score": 0.9953725
    },
    {
      "index": 2,
      "relevance_score": 0.002157342
    },
    {
      "index": 1,
      "relevance_score": 0.00046371284
    },
    {
      "index": 3,
      "relevance_score": 0.000017502925
    }
  ],
  "meta": {
    "tokens": {
      "input_tokens": 28
    }
  }
}`
            }
        case 11:
            return {
                title: t('modeType.11'),
                endpoint: '/parse/pdf',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/parse/pdf \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: multipart/form-data' \\
--form model=${modelConfig.model} \\
--form 'file=@"test.pdf"'`,
                responseExample: `{
  "pages": 1,
  "markdown": "sf ad fda daf da \\\\( f \\\\) ds f sd fs d afdas fsd asfad f\\n\\n\\n\\n![img](data:image/jpeg;base64,/9...)\\n\\n| sadsa |  |  |\\n| --- | --- | --- |\\n|  | sadasdsa | sad |\\n|  |  | dsadsadsa |\\n|  |  |  |\\n\\n\\n\\na fda"
}`
            }
        default:
            return {
                title: t('modeType.0'),
                endpoint: '',
                method: '',
                responseFormat: 'json',
                requestExample: 'unknown',
                responseExample: 'unknown'
            }
    }
}

const ApiDocDrawer: React.FC<ApiDocDrawerProps> = ({ isOpen, onClose, modelConfig }) => {
    const { t } = useTranslation()
    const apiDoc = getApiDocContent(modelConfig, "", t)

    return (
        <Sheet open={isOpen} onOpenChange={(open) => {
            if (!open) onClose()
        }}>
            <SheetContent 
                side="right"
                className="p-0 overflow-hidden w-[400px] max-w-md"
            >
                <div className="flex flex-col gap-3 overflow-y-auto p-6 pb-3 h-[calc(100%-48px)]">
                    {/* method */}
                    <div className="flex gap-2.5 items-center">
                        <Badge className="flex px-2 py-0.5 justify-center items-center gap-0.5 rounded bg-blue-100">
                            <span className="text-blue-600 font-medium text-xs leading-4 tracking-wide">
                                {apiDoc.method}
                            </span>
                        </Badge>
                        <svg
                            xmlns="http://www.w3.org/2000/svg"
                            width="2"
                            height="18"
                            viewBox="0 0 2 18"
                            fill="none">
                            <path d="M1 1L1 17" stroke="#F0F1F6" strokeLinecap="round" />
                        </svg>

                        <div className="flex gap-1 items-center justify-start">
                            {apiDoc.endpoint
                                .split('/')
                                .filter(Boolean)
                                .map((segment, index) => (
                                    <React.Fragment key={index}>
                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            width="5"
                                            height="12"
                                            viewBox="0 0 5 12"
                                            fill="none">
                                            <path
                                                d="M4.42017 1.30151L0.999965 10.6984"
                                                stroke="#C4CBD7"
                                                strokeLinecap="round"
                                            />
                                        </svg>
                                        <span className="text-gray-600 font-medium text-xs leading-4 tracking-wide">
                                            {segment}
                                        </span>
                                    </React.Fragment>
                                ))}
                        </div>
                    </div>

                    {/* request example and response example */}
                    <div className="flex flex-col gap-4 items-start w-full">
                        {/* request example */}
                        <div className="flex flex-col gap-2 items-start w-full">
                            <span className="text-gray-900 font-medium text-xs leading-4 tracking-wide">
                                {t('apiDoc.requestExample')}
                            </span>

                            {/* code */}
                            <div className="flex flex-col items-start justify-center w-full rounded-md overflow-hidden">
                                <div className="flex w-full p-2.5 justify-between items-center bg-[#232833]">
                                    <span className="text-white font-medium text-xs leading-4 tracking-wide">
                                        {'bash'}
                                    </span>

                                    <Button
                                        onClick={() => {
                                            navigator.clipboard.writeText(apiDoc.requestExample).then(
                                                () => {
                                                    toast.success(t('common.copied'))
                                                },
                                                (err) => {
                                                    toast.error(err?.message || t('common.copyFailed'))
                                                }
                                            )
                                        }}
                                        variant="ghost"
                                        size="icon"
                                        className="inline-flex p-1 min-w-0 h-[22px] w-[22px] justify-center items-center rounded bg-transparent hover:bg-white/10">
                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            width="14"
                                            height="14"
                                            viewBox="0 0 14 14"
                                            fill="none">
                                            <path
                                                fillRule="evenodd"
                                                clipRule="evenodd"
                                                d="M2.86483 2.30131C2.73937 2.30131 2.61904 2.35115 2.53032 2.43987C2.44161 2.52859 2.39176 2.64891 2.39176 2.77438V7.5282C2.39176 7.65366 2.44161 7.77399 2.53032 7.86271C2.61904 7.95142 2.73937 8.00127 2.86483 8.00127H3.39304C3.7152 8.00127 3.97637 8.26243 3.97637 8.5846C3.97637 8.90676 3.7152 9.16793 3.39304 9.16793H2.86483C2.42995 9.16793 2.01288 8.99517 1.70537 8.68766C1.39786 8.38015 1.2251 7.96308 1.2251 7.5282V2.77438C1.2251 2.3395 1.39786 1.92242 1.70537 1.61491C2.01288 1.3074 2.42995 1.13464 2.86483 1.13464H7.61865C8.05354 1.13464 8.47061 1.3074 8.77812 1.61491C9.08563 1.92242 9.25839 2.33949 9.25839 2.77438V3.30258C9.25839 3.62475 8.99722 3.88592 8.67505 3.88592C8.35289 3.88592 8.09172 3.62475 8.09172 3.30258V2.77438C8.09172 2.64891 8.04188 2.52859 7.95316 2.43987C7.86444 2.35115 7.74412 2.30131 7.61865 2.30131H2.86483ZM6.56225 5.99872C6.30098 5.99872 6.08918 6.21052 6.08918 6.47179V11.2256C6.08918 11.4869 6.30098 11.6987 6.56225 11.6987H11.3161C11.5773 11.6987 11.7891 11.4869 11.7891 11.2256V6.47179C11.7891 6.21052 11.5773 5.99872 11.3161 5.99872H6.56225ZM4.92251 6.47179C4.92251 5.56619 5.65664 4.83206 6.56225 4.83206H11.3161C12.2217 4.83206 12.9558 5.56619 12.9558 6.47179V11.2256C12.9558 12.1312 12.2217 12.8653 11.3161 12.8653H6.56225C5.65664 12.8653 4.92251 12.1312 4.92251 11.2256V6.47179Z"
                                                fill="white"
                                                fillOpacity="0.8"
                                            />
                                        </svg>
                                    </Button>
                                </div>
                                <div className="p-3 bg-[#14181E] w-full">
                                    <CodeBlock code={apiDoc.requestExample} language="bash" />
                                </div>
                            </div>

                            {/* additional info */}
                            {apiDoc?.requestAdditionalInfo?.voices &&
                                apiDoc?.requestAdditionalInfo?.voices?.length > 0 && (
                                    <div className="flex flex-col p-2.5 w-full gap-2 items-start rounded-md border border-gray-200">
                                        <div className="flex gap-2">
                                            <span className="text-blue-600 font-medium text-xs leading-4 tracking-wide">
                                                {'voice'}
                                            </span>
                                            <div className="flex gap-1">
                                                <Badge variant="outline" className="bg-gray-100 text-gray-500 font-medium text-xs">
                                                    {'enum<string>'}
                                                </Badge>
                                                <Badge className="bg-amber-50 text-amber-600 font-medium text-xs border-0">
                                                    {t('apiDoc.voice')}
                                                </Badge>
                                            </div>
                                        </div>

                                        <div className="flex flex-col gap-1 items-start w-full">
                                            <span className="text-gray-500 font-medium text-xs leading-4 tracking-wide">
                                                {t('apiDoc.voiceValues')}
                                            </span>

                                            <div className="flex flex-wrap gap-2">
                                                {apiDoc?.requestAdditionalInfo?.voices?.map((voice) => (
                                                    <Badge
                                                        key={voice}
                                                        variant="outline"
                                                        className="bg-gray-100 text-gray-900 font-medium text-xs">
                                                        {voice}
                                                    </Badge>
                                                ))}
                                            </div>
                                        </div>
                                    </div>
                                )}

                            {apiDoc?.requestAdditionalInfo?.formats &&
                                apiDoc?.requestAdditionalInfo?.formats?.length > 0 && (
                                    <div className="flex flex-col p-2.5 w-full gap-2 items-start rounded-md border border-gray-200">
                                        <div className="flex gap-2">
                                            <span className="text-blue-600 font-medium text-xs leading-4 tracking-wide">
                                                {'response_format'}
                                            </span>
                                            <div className="flex gap-1">
                                                <Badge variant="outline" className="bg-gray-100 text-gray-500 font-medium text-xs">
                                                    {'enum<string>'}
                                                </Badge>
                                                <Badge variant="outline" className="bg-gray-100 text-gray-500 font-medium text-xs">
                                                    {'default:mp3'}
                                                </Badge>
                                            </div>
                                        </div>

                                        <div className="flex flex-col gap-1 items-start w-full">
                                            <span className="text-gray-500 font-medium text-xs leading-4 tracking-wide">
                                                {t('apiDoc.responseFormatValues')}
                                            </span>

                                            <div className="flex flex-wrap gap-2">
                                                {apiDoc?.requestAdditionalInfo?.formats?.map((format) => (
                                                    <Badge
                                                        key={format}
                                                        variant="outline"
                                                        className="bg-gray-100 text-gray-900 font-medium text-xs">
                                                        {format}
                                                    </Badge>
                                                ))}
                                            </div>
                                        </div>
                                    </div>
                                )}
                        </div>

                        {/* response example */}
                        <div className="flex flex-col gap-2 items-start w-full">
                            <span className="text-gray-900 font-medium text-xs leading-4 tracking-wide">
                                {t('apiDoc.responseExample')}
                            </span>

                            {/* code */}
                            <div className="flex flex-col items-start justify-center w-full rounded-md overflow-hidden">
                                <div className="flex w-full p-2.5 justify-between items-center bg-[#232833]">
                                    <span className="text-white font-medium text-xs leading-4 tracking-wide">
                                        {apiDoc.responseFormat}
                                    </span>

                                    <Button
                                        onClick={() => {
                                            navigator.clipboard.writeText(apiDoc.responseExample).then(
                                                () => {
                                                    toast.success(t('common.copied'))
                                                },
                                                (err) => {
                                                    toast.error(err?.message || t('common.copyFailed'))
                                                }
                                            )
                                        }}
                                        variant="ghost"
                                        size="icon"
                                        className="inline-flex p-1 min-w-0 h-[22px] w-[22px] justify-center items-center rounded bg-transparent hover:bg-white/10">
                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            width="14"
                                            height="14"
                                            viewBox="0 0 14 14"
                                            fill="none">
                                            <path
                                                fillRule="evenodd"
                                                clipRule="evenodd"
                                                d="M2.86483 2.30131C2.73937 2.30131 2.61904 2.35115 2.53032 2.43987C2.44161 2.52859 2.39176 2.64891 2.39176 2.77438V7.5282C2.39176 7.65366 2.44161 7.77399 2.53032 7.86271C2.61904 7.95142 2.73937 8.00127 2.86483 8.00127H3.39304C3.7152 8.00127 3.97637 8.26243 3.97637 8.5846C3.97637 8.90676 3.7152 9.16793 3.39304 9.16793H2.86483C2.42995 9.16793 2.01288 8.99517 1.70537 8.68766C1.39786 8.38015 1.2251 7.96308 1.2251 7.5282V2.77438C1.2251 2.3395 1.39786 1.92242 1.70537 1.61491C2.01288 1.3074 2.42995 1.13464 2.86483 1.13464H7.61865C8.05354 1.13464 8.47061 1.3074 8.77812 1.61491C9.08563 1.92242 9.25839 2.33949 9.25839 2.77438V3.30258C9.25839 3.62475 8.99722 3.88592 8.67505 3.88592C8.35289 3.88592 8.09172 3.62475 8.09172 3.30258V2.77438C8.09172 2.64891 8.04188 2.52859 7.95316 2.43987C7.86444 2.35115 7.74412 2.30131 7.61865 2.30131H2.86483ZM6.56225 5.99872C6.30098 5.99872 6.08918 6.21052 6.08918 6.47179V11.2256C6.08918 11.4869 6.30098 11.6987 6.56225 11.6987H11.3161C11.5773 11.6987 11.7891 11.4869 11.7891 11.2256V6.47179C11.7891 6.21052 11.5773 5.99872 11.3161 5.99872H6.56225ZM4.92251 6.47179C4.92251 5.56619 5.65664 4.83206 6.56225 4.83206H11.3161C12.2217 4.83206 12.9558 5.56619 12.9558 6.47179V11.2256C12.9558 12.1312 12.2217 12.8653 11.3161 12.8653H6.56225C5.65664 12.8653 4.92251 12.1312 4.92251 11.2256V6.47179Z"
                                                fill="white"
                                                fillOpacity="0.8"
                                            />
                                        </svg>
                                    </Button>
                                </div>
                                <div className="p-3 bg-[#14181E] w-full">
                                    <CodeBlock code={apiDoc.responseExample} language={apiDoc.responseFormat === 'json' ? 'json' : 'text'} />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </SheetContent>
        </Sheet>
    )
}

export default ApiDocDrawer
