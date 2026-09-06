import React from 'react'
import {
    Sheet,
    SheetContent,
    SheetTitle,
} from "@/components/ui/sheet"
import { CopyButton } from "@/components/common/CopyButton"
import { Badge } from "@/components/ui/badge"
import { ModelConfig } from '@/types/model'
import { useTranslation } from 'react-i18next'
import CodeBlock from './CodeHight'
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
        case 2:
            return {
                title: t('modeType.2'),
                endpoint: '/completions',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/completions \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "prompt": "Write a short product tagline",
  "max_tokens": 64,
  "temperature": 0.7
}'`,
                responseExample: `{
  "object": "text_completion",
  "model": "${modelConfig.model}",
  "choices": [
    {
      "text": "Cloud-native apps, shipped faster.",
      "index": 0,
      "finish_reason": "stop"
    }
  ]
}`
            }
        case 4:
            return {
                title: t('modeType.4'),
                endpoint: '/moderations',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/moderations \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "input": "Text to classify"
}'`,
                responseExample: `{
  "id": "modr_123",
  "model": "${modelConfig.model}",
  "results": [
    {
      "flagged": false,
      "categories": {}
    }
  ]
}`
            }
        case 5:
            return {
                title: t('modeType.5'),
                endpoint: '/images/generations',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/images/generations \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "prompt": "A minimal cloud dashboard illustration",
  "size": "1024x1024",
  "n": 1
}'`,
                responseExample: `{
  "created": 1729672480,
  "data": [
    {
      "url": "https://example.com/image.png"
    }
  ]
}`
            }
        case 6:
            return {
                title: t('modeType.6'),
                endpoint: '/images/edits',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/images/edits \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: multipart/form-data' \\
--form model=${modelConfig.model} \\
--form 'image=@"image.png"' \\
--form 'prompt=Add a clean blue background' \\
--form size=1024x1024`,
                responseExample: `{
  "created": 1729672480,
  "data": [
    {
      "url": "https://example.com/edited-image.png"
    }
  ]
}`
            }
        case 9:
            return {
                title: t('modeType.9'),
                endpoint: '/audio/translations',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/audio/translations \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: multipart/form-data' \\
--form model=${modelConfig.model} \\
--form 'file=@"audio.mp3"'`,
                responseExample: `{
  "text": "Translated transcript text"
}`
            }
        case 12:
            return {
                title: t('modeType.12'),
                endpoint: '/messages',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/messages \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "max_tokens": 512,
  "messages": [
    {
      "role": "user",
      "content": "Summarize this release note"
    }
  ]
}'`,
                responseExample: `{
  "id": "msg_123",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Summary text"
    }
  ]
}`
            }
        case 13:
            return {
                title: t('modeType.13'),
                endpoint: '/video/generations/jobs',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/video/generations/jobs \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "prompt": "A calm ocean at sunrise",
  "width": 1280,
  "height": 720,
  "n_seconds": 5
}'`,
                responseExample: `{
  "id": "vgjob_123",
  "object": "video.generation.job",
  "status": "queued",
  "model": "${modelConfig.model}"
}`
            }
        case 14:
            return {
                title: t('modeType.14'),
                endpoint: '/video/generations/jobs/{id}',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1/video/generations/jobs/vgjob_123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "id": "vgjob_123",
  "object": "video.generation.job",
  "status": "succeeded",
  "generations": [
    {
      "id": "video_123"
    }
  ]
}`
            }
        case 15:
            return {
                title: t('modeType.15'),
                endpoint: '/video/generations/{id}/content/video',
                method: 'GET',
                responseFormat: 'binary',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1/video/generations/video_123/content/video \\
--header "Authorization: Bearer $token" \\
--output video.mp4`,
                responseExample: 'Binary video data'
            }
        case 16:
            return {
                title: t('modeType.16'),
                endpoint: '/responses',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/responses \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "input": "Write a concise status update"
}'`,
                responseExample: `{
  "id": "resp_123",
  "object": "response",
  "status": "completed",
  "output_text": "Status update text"
}`
            }
        case 17:
            return {
                title: t('modeType.17'),
                endpoint: '/responses/{response_id}',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1/responses/resp_123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "id": "resp_123",
  "object": "response",
  "status": "completed"
}`
            }
        case 18:
            return {
                title: t('modeType.18'),
                endpoint: '/responses/{response_id}',
                method: 'DELETE',
                responseFormat: 'json',
                requestExample: `curl --request DELETE \\
--url ${apiEndpoint}/v1/responses/resp_123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "id": "resp_123",
  "object": "response.deleted",
  "deleted": true
}`
            }
        case 19:
            return {
                title: t('modeType.19'),
                endpoint: '/responses/{response_id}/cancel',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/responses/resp_123/cancel \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "id": "resp_123",
  "object": "response",
  "status": "cancelled"
}`
            }
        case 20:
            return {
                title: t('modeType.20'),
                endpoint: '/responses/{response_id}/input_items',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1/responses/resp_123/input_items \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "object": "list",
  "data": [
    {
      "id": "item_123",
      "type": "message"
    }
  ]
}`
            }
        case 21:
            return {
                title: t('modeType.21'),
                endpoint: '/models/{model}:generateContent',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1beta/models/${modelConfig.model}:generateContent \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Explain Kubernetes in one sentence"
        }
      ]
    }
  ]
}'`,
                responseExample: `{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "Kubernetes automates deployment, scaling, and management of containerized applications."
          }
        ]
      }
    }
  ]
}`
            }
        case 22:
            return {
                title: t('modeType.22'),
                endpoint: '/videos',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/videos \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "prompt": "A calm ocean at sunrise",
  "seconds": 5,
  "size": "1280x720"
}'`,
                responseExample: `{
  "id": "video_123",
  "object": "video",
  "status": "queued",
  "model": "${modelConfig.model}"
}`
            }
        case 23:
            return {
                title: t('modeType.23'),
                endpoint: '/videos/{video_id}',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1/videos/video_123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "id": "video_123",
  "object": "video",
  "status": "completed"
}`
            }
        case 24:
            return {
                title: t('modeType.24'),
                endpoint: '/videos/{video_id}/content',
                method: 'GET',
                responseFormat: 'binary',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1/videos/video_123/content \\
--header "Authorization: Bearer $token" \\
--output video.mp4`,
                responseExample: 'Binary video data'
            }
        case 25:
            return {
                title: t('modeType.25'),
                endpoint: '/videos/{video_id}',
                method: 'DELETE',
                responseFormat: 'text',
                requestExample: `curl --request DELETE \\
--url ${apiEndpoint}/v1/videos/video_123 \\
--header "Authorization: Bearer $token"`,
                responseExample: 'No content'
            }
        case 26:
            return {
                title: t('modeType.26'),
                endpoint: '/videos/{video_id}/remix',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/videos/video_123/remix \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "prompt": "Make it cinematic",
  "seconds": 5,
  "size": "1280x720"
}'`,
                responseExample: `{
  "id": "video_456",
  "object": "video",
  "status": "queued"
}`
            }
        case 27:
            return {
                title: t('modeType.27'),
                endpoint: '/models/{model}:predictLongRunning',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1beta/models/${modelConfig.model}:predictLongRunning \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "instances": [
    {
      "prompt": "A calm ocean at sunrise"
    }
  ],
  "parameters": {
    "durationSeconds": 8,
    "resolution": "720p"
  }
}'`,
                responseExample: `{
  "name": "operations/video-operation-123",
  "done": false
}`
            }
        case 28:
            return {
                title: t('modeType.28'),
                endpoint: '/operations/{operation_id}',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1beta/operations/video-operation-123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "name": "operations/video-operation-123",
  "done": true,
  "response": {}
}`
            }
        case 29:
            return {
                title: t('modeType.29'),
                endpoint: '/models/{model}:generateContent',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1beta/models/${modelConfig.model}:generateContent \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "contents": [
    {
      "parts": [
        {
          "text": "Say hello in a friendly voice"
        }
      ]
    }
  ],
  "generationConfig": {
    "responseModalities": ["AUDIO"]
  }
}'`,
                responseExample: `{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "inlineData": {
              "mimeType": "audio/wav",
              "data": "BASE64_AUDIO"
            }
          }
        ]
      }
    }
  ]
}`
            }
        case 30:
            return {
                title: t('modeType.30'),
                endpoint: '/models/{model}:generateContent',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1beta/models/${modelConfig.model}:generateContent \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "contents": [
    {
      "parts": [
        {
          "text": "Generate a clean product icon"
        }
      ]
    }
  ],
  "generationConfig": {
    "responseModalities": ["IMAGE"]
  }
}'`,
                responseExample: `{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "inlineData": {
              "mimeType": "image/png",
              "data": "BASE64_IMAGE"
            }
          }
        ]
      }
    }
  ]
}`
            }
        case 31:
            return {
                title: t('modeType.31'),
                endpoint: '/files/{file}:download',
                method: 'GET',
                responseFormat: 'binary',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/v1beta/files/abc123:download?alt=media \\
--header "Authorization: Bearer $token" \\
--output video.mp4`,
                responseExample: 'Binary file data'
            }
        case 32:
            return {
                title: t('modeType.32'),
                endpoint: '/videos/edits',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/videos/edits \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: multipart/form-data' \\
--form model=${modelConfig.model} \\
--form 'video=@"source.mp4"' \\
--form 'prompt=Replace the background with a sunrise' \\
--form seconds=5 \\
--form size=1280x720`,
                responseExample: `{
  "id": "video_edited_123",
  "object": "video",
  "status": "queued"
}`
            }
        case 33:
            return {
                title: t('modeType.33'),
                endpoint: '/videos/extensions',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/v1/videos/extensions \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: multipart/form-data' \\
--form model=${modelConfig.model} \\
--form 'video=@"source.mp4"' \\
--form 'prompt=Continue the scene naturally' \\
--form seconds=5 \\
--form size=1280x720`,
                responseExample: `{
  "id": "video_extended_123",
  "object": "video",
  "status": "queued"
}`
            }
        case 34:
            return {
                title: t('modeType.34'),
                endpoint: '/services/aigc/video-generation/video-synthesis',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/api/v1/services/aigc/video-generation/video-synthesis \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "input": {
    "prompt": "A calm ocean at sunrise"
  },
  "parameters": {
    "duration": 5,
    "size": "720P"
  }
}'`,
                responseExample: `{
  "output": {
    "task_id": "ali-task-123",
    "task_status": "PENDING"
  },
  "request_id": "request-123"
}`
            }
        case 35:
            return {
                title: t('modeType.35'),
                endpoint: '/tasks/{task_id}',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/api/v1/tasks/ali-task-123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "output": {
    "task_id": "ali-task-123",
    "task_status": "SUCCEEDED",
    "video_url": "https://example.com/video.mp4"
  }
}`
            }
        case 36:
            return {
                title: t('modeType.36'),
                endpoint: '/contents/generations/tasks',
                method: 'POST',
                responseFormat: 'json',
                requestExample: `curl --request POST \\
--url ${apiEndpoint}/api/v3/contents/generations/tasks \\
--header "Authorization: Bearer $token" \\
--header 'Content-Type: application/json' \\
--data '{
  "model": "${modelConfig.model}",
  "content": [
    {
      "type": "text",
      "text": "A calm ocean at sunrise"
    }
  ],
  "duration": 5,
  "resolution": "720p",
  "ratio": "16:9"
}'`,
                responseExample: `{
  "id": "doubao-task-123",
  "model": "${modelConfig.model}",
  "status": "queued"
}`
            }
        case 37:
            return {
                title: t('modeType.37'),
                endpoint: '/contents/generations/tasks/{task_id}',
                method: 'GET',
                responseFormat: 'json',
                requestExample: `curl --request GET \\
--url ${apiEndpoint}/api/v3/contents/generations/tasks/doubao-task-123 \\
--header "Authorization: Bearer $token"`,
                responseExample: `{
  "id": "doubao-task-123",
  "status": "succeeded",
  "content": {
    "video_url": "https://example.com/video.mp4"
  }
}`
            }
        case 38:
            return {
                title: t('modeType.38'),
                endpoint: '/contents/generations/tasks/{task_id}',
                method: 'DELETE',
                responseFormat: 'text',
                requestExample: `curl --request DELETE \\
--url ${apiEndpoint}/api/v3/contents/generations/tasks/doubao-task-123 \\
--header "Authorization: Bearer $token"`,
                responseExample: 'No content'
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
                aria-describedby={undefined}
                className="w-full max-w-full overflow-hidden p-0 sm:w-[560px] sm:max-w-[560px]"
            >
                <div className="flex h-full min-w-0 flex-col gap-4 overflow-y-auto p-4 pt-12 sm:p-6 sm:pt-12">
                    <SheetTitle className="text-base">{modelConfig.model}</SheetTitle>
                    {/* method */}
                    <div className="flex gap-2.5 items-center">
                        <Badge className="flex px-2 py-0.5 justify-center items-center gap-0.5 rounded bg-blue-100 dark:bg-blue-900">
                            <span className="text-blue-600 dark:text-blue-300 font-medium text-xs leading-4">
                                {apiDoc.method}
                            </span>
                        </Badge>
                        <svg
                            xmlns="http://www.w3.org/2000/svg"
                            width="2"
                            height="18"
                            viewBox="0 0 2 18"
                            fill="none">
                            <path d="M1 1L1 17" stroke="#F0F1F6" strokeLinecap="round" className="dark:stroke-zinc-700" />
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
                                                className="dark:stroke-zinc-600"
                                            />
                                        </svg>
                                        <span className="text-gray-600 dark:text-gray-400 font-medium text-xs leading-4">
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
                            <span className="text-gray-900 dark:text-gray-100 font-medium text-xs leading-4">
                                {t('apiDoc.requestExample')}
                            </span>

                            {/* code */}
                            <div className="flex flex-col items-start justify-center w-full rounded-md overflow-hidden">
                                <div className="flex w-full p-2.5 justify-between items-center bg-[#232833]">
                                    <span className="text-white font-medium text-xs leading-4">
                                        {'bash'}
                                    </span>

                                    <CopyButton text={apiDoc.requestExample} className="size-8 text-white/80 hover:bg-white/10 hover:text-white" />
                                </div>
                                <div className="p-3 bg-[#14181E] w-full">
                                    <CodeBlock code={apiDoc.requestExample} language="bash" />
                                </div>
                            </div>

                            {/* additional info */}
                            {apiDoc?.requestAdditionalInfo?.voices &&
                                apiDoc?.requestAdditionalInfo?.voices?.length > 0 && (
                                    <div className="flex flex-col p-2.5 w-full gap-2 items-start rounded-md border border-gray-200 dark:border-gray-700">
                                        <div className="flex gap-2">
                                            <span className="text-blue-600 dark:text-blue-400 font-medium text-xs leading-4">
                                                {'voice'}
                                            </span>
                                            <div className="flex gap-1">
                                                <Badge variant="outline" className="bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400 font-medium text-xs">
                                                    {'enum<string>'}
                                                </Badge>
                                                <Badge className="bg-amber-50 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400 font-medium text-xs border-0">
                                                    {t('apiDoc.voice')}
                                                </Badge>
                                            </div>
                                        </div>

                                        <div className="flex flex-col gap-1 items-start w-full">
                                            <span className="text-gray-500 dark:text-gray-400 font-medium text-xs leading-4">
                                                {t('apiDoc.voiceValues')}
                                            </span>

                                            <div className="flex flex-wrap gap-2">
                                                {apiDoc?.requestAdditionalInfo?.voices?.map((voice) => (
                                                    <Badge
                                                        key={voice}
                                                        variant="outline"
                                                        className="bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-300 font-medium text-xs">
                                                        {voice}
                                                    </Badge>
                                                ))}
                                            </div>
                                        </div>
                                    </div>
                                )}

                            {apiDoc?.requestAdditionalInfo?.formats &&
                                apiDoc?.requestAdditionalInfo?.formats?.length > 0 && (
                                    <div className="flex flex-col p-2.5 w-full gap-2 items-start rounded-md border border-gray-200 dark:border-gray-700">
                                        <div className="flex gap-2">
                                            <span className="text-blue-600 dark:text-blue-400 font-medium text-xs leading-4">
                                                {'response_format'}
                                            </span>
                                            <div className="flex gap-1">
                                                <Badge variant="outline" className="bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400 font-medium text-xs">
                                                    {'enum<string>'}
                                                </Badge>
                                                <Badge variant="outline" className="bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400 font-medium text-xs">
                                                    {'default:mp3'}
                                                </Badge>
                                            </div>
                                        </div>

                                        <div className="flex flex-col gap-1 items-start w-full">
                                            <span className="text-gray-500 dark:text-gray-400 font-medium text-xs leading-4">
                                                {t('apiDoc.responseFormatValues')}
                                            </span>

                                            <div className="flex flex-wrap gap-2">
                                                {apiDoc?.requestAdditionalInfo?.formats?.map((format) => (
                                                    <Badge
                                                        key={format}
                                                        variant="outline"
                                                        className="bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-300 font-medium text-xs">
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
                            <span className="text-gray-900 dark:text-gray-100 font-medium text-xs leading-4">
                                {t('apiDoc.responseExample')}
                            </span>

                            {/* code */}
                            <div className="flex flex-col items-start justify-center w-full rounded-md overflow-hidden">
                                <div className="flex w-full p-2.5 justify-between items-center bg-[#232833]">
                                    <span className="text-white font-medium text-xs leading-4">
                                        {apiDoc.responseFormat}
                                    </span>

                                    <CopyButton text={apiDoc.responseExample} className="size-8 text-white/80 hover:bg-white/10 hover:text-white" />
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
