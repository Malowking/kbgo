import { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import rehypeRaw from 'rehype-raw';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus, vs } from 'react-syntax-highlighter/dist/esm/styles/prism';
import mermaid from 'mermaid';
import { Copy, Check } from 'lucide-react';
import 'katex/dist/katex.min.css';

interface MessageContentProps {
  content: string;
  reasoningContent?: string;
  isStreaming?: boolean;
  theme?: 'light' | 'dark';
}

// Initialize mermaid
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'loose',
  fontFamily: 'system-ui, -apple-system, sans-serif',
});

export default function MessageContent({ content, reasoningContent, isStreaming = false, theme = 'light' }: MessageContentProps) {
  const [copiedCode, setCopiedCode] = useState<string | null>(null);

  const copyToClipboard = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedCode(id);
      setTimeout(() => setCopiedCode(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  return (
    <div className={`message-content prose max-w-none ${theme === 'dark' ? 'prose-invert' : ''}`}>
      {/* Reasoning Content (思考过程) - 灰色显示 */}
      {reasoningContent && (
        <div className="mb-4 p-3 bg-gray-50 border-l-4 border-gray-400 rounded-r">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-xs font-semibold text-gray-500 uppercase">💭 思考过程</span>
          </div>
          <div className="text-gray-600 text-sm">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                p({ children }) {
                  return <p className="my-1 leading-relaxed">{children}</p>;
                },
                code({ inline, children }: any) {
                  if (inline) {
                    return <code className="px-1 py-0.5 rounded bg-gray-200 text-gray-700 text-xs font-mono">{children}</code>;
                  }
                  return <code className="block p-2 bg-gray-200 rounded text-xs font-mono overflow-x-auto">{children}</code>;
                },
              }}
            >
              {reasoningContent}
            </ReactMarkdown>
          </div>
        </div>
      )}

      {/* Main Content (实际回答) */}
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex, rehypeRaw]}
        components={{
          // Code blocks
          code({ node, inline, className, children, ...props }: any) {
            const match = /language-(\w+)/.exec(className || '');
            const language = match ? match[1] : '';
            const codeString = String(children).replace(/\n$/, '');
            const codeId = `code-${Math.random().toString(36).substr(2, 9)}`;

            // Handle Mermaid diagrams
            if (language === 'mermaid' && !inline) {
              return <MermaidDiagram chart={codeString} isStreaming={isStreaming} />;
            }

            // Inline code
            if (inline) {
              return (
                <code className="px-1.5 py-0.5 rounded bg-gray-100 text-red-600 text-sm font-mono" {...props}>
                  {children}
                </code>
              );
            }

            // Code block with syntax highlighting
            return (
              <div className="relative group my-4">
                <div className="flex items-center justify-between bg-gray-800 text-gray-200 px-4 py-2 rounded-t-lg text-sm">
                  <span className="font-medium">{language || 'text'}</span>
                  <button
                    onClick={() => copyToClipboard(codeString, codeId)}
                    className="flex items-center gap-1 px-2 py-1 rounded hover:bg-gray-700 transition-colors"
                    title="Copy code"
                  >
                    {copiedCode === codeId ? (
                      <>
                        <Check className="w-4 h-4" />
                        <span className="text-xs">Copied!</span>
                      </>
                    ) : (
                      <>
                        <Copy className="w-4 h-4" />
                        <span className="text-xs">Copy</span>
                      </>
                    )}
                  </button>
                </div>
                <SyntaxHighlighter
                  style={theme === 'dark' ? vscDarkPlus : vs}
                  language={language || 'text'}
                  PreTag="div"
                  customStyle={{
                    margin: 0,
                    borderTopLeftRadius: 0,
                    borderTopRightRadius: 0,
                    borderBottomLeftRadius: '0.5rem',
                    borderBottomRightRadius: '0.5rem',
                  } as any}
                  {...props}
                >
                  {codeString}
                </SyntaxHighlighter>
              </div>
            );
          },

          // Images
          img({ src, alt, ...props }) {
            return (
              <div className="my-4">
                <img
                  src={src}
                  alt={alt}
                  className="max-w-full h-auto rounded-lg shadow-md"
                  loading="lazy"
                  {...props}
                />
                {alt && <p className="text-sm text-gray-500 mt-2 text-center italic">{alt}</p>}
              </div>
            );
          },

          // Videos
          video({ src, ...props }) {
            return (
              <div className="my-4">
                <video
                  src={src}
                  controls
                  className="max-w-full h-auto rounded-lg shadow-md"
                  {...props}
                />
              </div>
            );
          },

          // Audio
          audio({ src, ...props }) {
            return (
              <div className="my-4">
                <audio src={src} controls className="w-full" {...props} />
              </div>
            );
          },

          // Links
          a({ href, children, ...props }) {
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:text-blue-800 underline"
                {...props}
              >
                {children}
              </a>
            );
          },

          // Tables
          table({ children, ...props }) {
            return (
              <div className="overflow-x-auto my-4">
                <table className="min-w-full border-collapse border border-gray-300" {...props}>
                  {children}
                </table>
              </div>
            );
          },

          th({ children, ...props }) {
            return (
              <th className="border border-gray-300 bg-gray-100 px-4 py-2 text-left font-semibold" {...props}>
                {children}
              </th>
            );
          },

          td({ children, ...props }) {
            return (
              <td className="border border-gray-300 px-4 py-2" {...props}>
                {children}
              </td>
            );
          },

          // Blockquotes
          blockquote({ children, ...props }) {
            return (
              <blockquote
                className="border-l-4 border-blue-500 pl-4 py-2 my-4 bg-blue-50 italic"
                {...props}
              >
                {children}
              </blockquote>
            );
          },

          // Lists
          ul({ children, ...props }) {
            return (
              <ul className="list-disc list-inside my-2 space-y-1" {...props}>
                {children}
              </ul>
            );
          },

          ol({ children, ...props }) {
            return (
              <ol className="list-decimal list-inside my-2 space-y-1" {...props}>
                {children}
              </ol>
            );
          },

          // Headings
          h1({ children, ...props }) {
            return (
              <h1 className="text-3xl font-bold mt-6 mb-4" {...props}>
                {children}
              </h1>
            );
          },

          h2({ children, ...props }) {
            return (
              <h2 className="text-2xl font-bold mt-5 mb-3" {...props}>
                {children}
              </h2>
            );
          },

          h3({ children, ...props }) {
            return (
              <h3 className="text-xl font-bold mt-4 mb-2" {...props}>
                {children}
              </h3>
            );
          },

          // Paragraphs
          p({ children, ...props }) {
            return (
              <p className="my-2 leading-relaxed" {...props}>
                {children}
              </p>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>

      {/* Typing cursor for streaming */}
      {isStreaming && (
        <span className="inline-block w-2 h-4 ml-1 bg-blue-500 animate-pulse" />
      )}
    </div>
  );
}

// Mermaid Diagram Component
function MermaidDiagram({ chart, isStreaming = false }: { chart: string; isStreaming?: boolean }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [svg, setSvg] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [isRendering, setIsRendering] = useState(true);
  const renderTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    // Clear any pending render timeout
    if (renderTimeoutRef.current) {
      clearTimeout(renderTimeoutRef.current);
    }

    const renderDiagram = async () => {
      if (!chart) return;

      // Check if the chart looks incomplete (basic validation)
      const trimmedChart = chart.trim();
      const hasGraphDeclaration = /^(graph|flowchart|sequenceDiagram|classDiagram|stateDiagram|erDiagram|gantt|pie|journey)/i.test(trimmedChart);

      if (!hasGraphDeclaration) {
        setIsRendering(true);
        return;
      }

      setIsRendering(true);
      setError('');

      try {
        // Clean and validate the chart code
        let cleanedChart = trimmedChart;

        // Remove any markdown code fence artifacts
        cleanedChart = cleanedChart.replace(/^```mermaid\n?/i, '').replace(/\n?```$/i, '');

        // Fix common syntax issues
        // 1. Fix node labels with numbers that aren't properly quoted
        cleanedChart = cleanedChart.replace(/\[([^\]]*\d[^\]]*)\]/g, (match, content) => {
          // If content contains numbers and special chars, ensure it's properly formatted
          if (/\d/.test(content) && !/^["'].*["']$/.test(content)) {
            // Escape special characters
            const escaped = content.replace(/[|]/g, '');
            return `["${escaped}"]`;
          }
          return match;
        });

        // 2. Fix arrow labels with numbers
        cleanedChart = cleanedChart.replace(/\|([^|]*\d[^|]*)\|/g, (match, content) => {
          // Ensure arrow labels are properly quoted
          if (!/^["'].*["']$/.test(content.trim())) {
            return `|"${content.trim()}"|`;
          }
          return match;
        });

        // Generate unique ID for mermaid
        const id = `mermaid-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

        // Render mermaid diagram
        const result = await mermaid.render(id, cleanedChart);
        setSvg(result.svg);
        setError('');
      } catch (err: any) {
        // Only log errors if not streaming (to avoid console spam during partial renders)
        if (!isStreaming) {
          console.error('Mermaid rendering error:', err);
          setError(err.message || 'Failed to render diagram');
        } else {
          // During streaming, silently fail and keep showing loading state
          setError('');
        }
      } finally {
        setIsRendering(false);
      }
    };

    // If streaming, debounce the render to avoid rendering incomplete diagrams
    if (isStreaming) {
      renderTimeoutRef.current = setTimeout(() => {
        renderDiagram();
      }, 500); // Wait 500ms after last update before rendering
    } else {
      // If not streaming, render immediately
      renderDiagram();
    }

    return () => {
      if (renderTimeoutRef.current) {
        clearTimeout(renderTimeoutRef.current);
      }
    };
  }, [chart, isStreaming]);

  if (error) {
    return (
      <div className="my-4 p-4 bg-red-50 border border-red-200 rounded-lg">
        <p className="text-red-600 text-sm font-semibold">Mermaid 渲染失败</p>
        <p className="text-red-500 text-xs mt-1">{error}</p>
        <details className="mt-2">
          <summary className="text-xs text-gray-600 cursor-pointer">查看代码</summary>
          <pre className="mt-2 text-xs text-gray-700 overflow-x-auto bg-white p-2 rounded border">
            {chart}
          </pre>
        </details>
      </div>
    );
  }

  if (isRendering || !svg) {
    return (
      <div className="my-4 p-4 bg-gray-50 border border-gray-200 rounded-lg">
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
          <p className="text-gray-500 text-sm">正在渲染 Mermaid 图表...</p>
        </div>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="my-4 p-4 bg-white border border-gray-200 rounded-lg overflow-x-auto"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}