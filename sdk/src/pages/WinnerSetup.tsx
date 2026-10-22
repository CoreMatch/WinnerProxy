import { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Typography,
} from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { getRealBackendUrl, buildServiceUrl } from '../utils/proxy';

function CodeBlock({ children }: { children: string }) {
  return (
    <Box
      sx={{
        position: 'relative',
        bgcolor: 'grey.900',
        color: 'grey.100',
        p: 2,
        borderRadius: 2,
        fontFamily: 'monospace',
        fontSize: '0.9rem',
        whiteSpace: 'pre-wrap',
        border: '1px solid',
        borderColor: 'grey.800',
      }}
    >
      {children}
    </Box>
  );
}

export default function WinnerSetup() {
  const [baseUrl, setBaseUrl] = useState('');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let mounted = true;

    // First check if the host has provided a service-specific SDK URL via manifest/presence
    // @ts-ignore
    const serviceSdkUrl = window.__HRP_SDK_CONFIG__?.winnerproxy?.sdk_url;

    if (serviceSdkUrl) {
      if (mounted) {
        setBaseUrl(serviceSdkUrl.replace(/\/$/, ''));
      }
    } else {
      // Fallback to dynamic relay-based URL calculation
      getRealBackendUrl().then((url: string) => {
        if (mounted) {
          setBaseUrl(buildServiceUrl(url, '').replace(/\/$/, ''));
        }
      });
    }

    return () => {
      mounted = false;
    };
  }, []);

  const handleCopy = async () => {
    if (!baseUrl) return;
    await navigator.clipboard.writeText(baseUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Box sx={{ maxWidth: 800, mx: 'auto', p: 3, display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Typography variant="h5" component="h1">
        WinnerProxy 配置
      </Typography>

      <Typography variant="body1">
        WinnerProxy 是一个支持 HRPAuth 与 Mojang 双回源的 Yggdrasil 代理服务。
        使用此代理地址，你可以同时登录 HRPAuth 账号与正版 Mojang 账号。
      </Typography>

      <Card>
        <CardContent>
          <Typography variant="subtitle2" color="text.secondary" gutterBottom>
            WinnerProxy API 地址
          </Typography>
          <CodeBlock>{baseUrl}</CodeBlock>
          <Button
            variant="contained"
            startIcon={<ContentCopyIcon />}
            onClick={handleCopy}
            sx={{ mt: 2 }}
          >
            {copied ? '已复制!' : '复制地址'}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ mb: 1 }}>
            使用说明
          </Typography>
          <Box component="ol" sx={{ pl: 2, m: 0 }}>
            <li>
              <Typography variant="body2" sx={{ mb: 1 }}>
                使用Authlib Injector或Loki. </Typography>
            </li>
            <li>
              <Typography variant="body2" sx={{ mb: 1 }}>
                Use Winnerproxy as upstream.
              </Typography>
            </li>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
