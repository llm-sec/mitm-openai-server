import React, { useMemo } from 'react';
import { Space, Card } from 'antd';
import { CommentOutlined } from '@ant-design/icons';
import JsonCardView from './JsonCardView';
import '../RequestDetailPage.css';

interface ResponseContentProps {
  responseHeaders: any;
  responseBody?: any;
  copyToClipboard: (text: any) => void;
  jsonPrettyTheme: any;
}

// 从响应体中提取 AI 回复文本（OpenAI / Anthropic 非流式 JSON）
const extractAssistantContent = (body: any): string => {
  if (!body || typeof body !== 'object' || Array.isArray(body)) {
    return '';
  }

  // OpenAI Chat Completions: choices[].message.content
  if (Array.isArray(body.choices)) {
    const parts: string[] = [];
    for (const choice of body.choices) {
      const msg = choice && choice.message;
      let content = '';
      if (msg && typeof msg.content === 'string') {
        content = msg.content;
      } else if (msg && Array.isArray(msg.content)) {
        // Anthropic 风格内容块
        content = msg.content
          .filter((block: any) => block && block.type === 'text' && block.text)
          .map((block: any) => block.text)
          .join('\n');
      } else if (choice && typeof choice.text === 'string') {
        // Completions API
        content = choice.text;
      }
      if (content) {
        parts.push(content);
      }
    }
    return parts.join('\n\n');
  }

  // Anthropic Messages: content 文本块数组
  if (Array.isArray(body.content)) {
    return body.content
      .filter((block: any) => block && (block.type === 'text' || block.type === undefined) && block.text)
      .map((block: any) => block.text)
      .join('\n');
  }

  // 直接文本字段
  if (typeof body.text === 'string') {
    return body.text;
  }

  return '';
};

const ResponseContent: React.FC<ResponseContentProps> = ({
  responseHeaders,
  responseBody,
  copyToClipboard,
  jsonPrettyTheme
}) => {
  // 确保响应头正确显示
  const formattedHeaders = useMemo(() => {
    if (!responseHeaders) return {};

    // 如果是字符串，尝试解析为JSON
    if (typeof responseHeaders === 'string') {
      try {
        return JSON.parse(responseHeaders);
      } catch {
        return { raw: responseHeaders };
      }
    }

    return responseHeaders;
  }, [responseHeaders]);

  // 确保响应体正确显示
  const formattedBody = useMemo(() => {
    if (!responseBody) return null;

    // 如果是字符串，尝试解析为JSON
    if (typeof responseBody === 'string') {
      try {
        return JSON.parse(responseBody);
      } catch {
        // 如果不是有效的JSON，直接返回原始字符串
        return responseBody;
      }
    }

    return responseBody;
  }, [responseBody]);

  // 提取 AI 回复摘要文本
  const assistantContent = useMemo(
    () => extractAssistantContent(formattedBody),
    [formattedBody]
  );

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      {assistantContent && (
        <Card className="ai-reply-card" styles={{ body: { padding: '16px 20px' } }}>
          <div className="ai-reply-head">
            <CommentOutlined />
            <span>AI 回复内容</span>
          </div>
          <div className="ai-reply-content">{assistantContent}</div>
        </Card>
      )}

      <JsonCardView
        title="响应头"
        data={formattedHeaders}
        copyToClipboard={copyToClipboard}
        jsonPrettyTheme={jsonPrettyTheme}
      />

      {formattedBody && (
        <JsonCardView
          title="响应体"
          data={formattedBody}
          copyToClipboard={copyToClipboard}
          jsonPrettyTheme={jsonPrettyTheme}
        />
      )}
    </Space>
  );
};

export default ResponseContent; 