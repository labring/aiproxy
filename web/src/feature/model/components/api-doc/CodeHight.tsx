import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter'
import bash from 'react-syntax-highlighter/dist/esm/languages/prism/bash'
import json from 'react-syntax-highlighter/dist/esm/languages/prism/json'
import { atomDark } from 'react-syntax-highlighter/dist/esm/styles/prism'

SyntaxHighlighter.registerLanguage('bash', bash)
SyntaxHighlighter.registerLanguage('json', json)

const CodeBlock = ({ code, language = 'bash' }: { code: string; language?: string }) => {
    const customizedStyle = {
        ...atomDark,
        'pre[class*="language-"]': {
            ...atomDark['pre[class*="language-"]'],
            backgroundColor: 'transparent',
            margin: 0,
            padding: 0
        }
    }

    return (
        <div className="min-w-0 overflow-x-auto">
            <SyntaxHighlighter
                language={language}
                style={customizedStyle}
                customStyle={{
                    fontSize: '12px',
                    overflowX: 'auto'
                }}
                codeTagProps={{
                    style: {
                        color: 'white'
                    }
                }}
                wrapLines={false}
                lineProps={{ style: { whiteSpace: 'pre' } }}>
                {code}
            </SyntaxHighlighter>
        </div>
    )
}

export default CodeBlock
