import { useState, useRef, useCallback, useEffect, useMemo } from 'react';
import { useSSE, SSEMessage } from './useSSE';

export interface LogLine {
  step: string;
  content: string;
  timestamp: number;
}

export interface UseLogStreamOptions {
  onLogUpdate?: (logs: string) => void;
  onError?: (error: string) => void;
  onStepChange?: (step: string) => void;
}

export interface UseLogStreamReturn {
  logs: string;
  logLines: LogLine[];
  isStreaming: boolean;
  isConnected: boolean;
  logStreamError: string | null;
  currentStep: string | null;
  startStream: (buildName: string) => void;
  stopStream: () => void;
  clearLogs: () => void;
}

export const useLogStream = (options: UseLogStreamOptions = {}): UseLogStreamReturn => {
  const { onLogUpdate, onError, onStepChange } = options;

  // Create stable refs for callbacks to avoid recreating useSSE hook
  const onLogUpdateRef = useRef(onLogUpdate);
  const onErrorRef = useRef(onError);
  const onStepChangeRef = useRef(onStepChange);

  // Update refs when callbacks change
  useEffect(() => {
    onLogUpdateRef.current = onLogUpdate;
    onErrorRef.current = onError;
    onStepChangeRef.current = onStepChange;
  }, [onLogUpdate, onError, onStepChange]);

  const [logs, setLogs] = useState<string>('');
  const [logLines, setLogLines] = useState<LogLine[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [logStreamError, setLogStreamError] = useState<string | null>(null);
  const [currentStep, setCurrentStep] = useState<string | null>(null);
  const [sseUrl, setSseUrl] = useState<string | null>(null);

  const logsRef = useRef<string>('');
  const logLinesRef = useRef<LogLine[]>([]);
  const currentStepRef = useRef<string | null>(null);

  const handleSSEMessage = useCallback((message: SSEMessage) => {
    const { event, data, id } = message;

    console.log('useLogStream: Received SSE message:', { event, data, id });

    if (data === 'Log stream connected') {
      console.log('useLogStream: Got connected message, event type:', event);
    }

    switch (event) {
      case 'connected':
        setIsStreaming(true);
        setLogStreamError(null);
        console.log('useLogStream: Connected to stream');
        break;

      case 'completed':
        setIsStreaming(false);
        onLogUpdateRef.current?.(logsRef.current);
        console.log('useLogStream: Stream completed');
        break;

      case 'disconnected':
        setIsStreaming(false);
        console.log('useLogStream: Stream disconnected');
        break;

      case 'step':
        if (id) {
          currentStepRef.current = id;
          setCurrentStep(id);
          onStepChangeRef.current?.(id);
          console.log('useLogStream: New step:', id);
        }
        break;

      case 'log':
        if (data) {
          // Use ref to get current step to avoid dependency on currentStep state
          const step = id || currentStepRef.current || 'unknown';
          const logLine: LogLine = {
            step,
            content: data,
            timestamp: Date.now(),
          };

          logLinesRef.current.push(logLine);
          setLogLines([...logLinesRef.current]);

          logsRef.current += data + '\n';
          setLogs(logsRef.current);
          onLogUpdateRef.current?.(logsRef.current);
        }
        break;

      case 'error':
        const errorMsg = data || 'Unknown error occurred';
        setLogStreamError(errorMsg);
        onErrorRef.current?.(errorMsg);
        console.error('useLogStream: Error event:', errorMsg);
        break;

      case 'waiting':
        console.log('useLogStream: Waiting for logs:', data);
        // Could show a waiting indicator here
        break;

      case 'ping':
        // Server keepalive, no action needed
        break;

      default:
        console.log('useLogStream: Unknown event type:', event, data);
        break;
    }
  }, []);

  const handleSSEError = useCallback((error: Event) => {
    console.error('useLogStream: SSE error:', error);
    setLogStreamError('Connection error');
    onErrorRef.current?.('Connection error');
  }, []);

  const handleSSEOpen = useCallback(() => {
    console.log('useLogStream: SSE connection opened');
    setLogStreamError(null);
  }, []);

  const handleSSEClose = useCallback(() => {
    console.log('useLogStream: SSE connection closed');
    setIsStreaming(false);
  }, []);

  const sseOptions = useMemo(() => {
    console.log('useLogStream: Creating new SSE options object');
    return {
      onMessage: handleSSEMessage,
      onError: handleSSEError,
      onOpen: handleSSEOpen,
      onClose: handleSSEClose,
      autoReconnect: false,
      maxReconnectAttempts: 0,
    };
  }, [handleSSEMessage, handleSSEError, handleSSEOpen, handleSSEClose]);

  const { isConnected, isConnecting } = useSSE(sseUrl, sseOptions);

  const startStream = useCallback((buildName: string) => {
    console.log('useLogStream: startStream called with buildName:', buildName);
    const newUrl = `/v1/builds/${buildName}/logs/sse`;
    if (sseUrl !== newUrl) {
      console.log('useLogStream: Setting new SSE URL:', newUrl);
      setSseUrl(newUrl);
    } else {
      console.log('useLogStream: URL unchanged, not reconnecting');
    }
  }, [sseUrl]);

  const stopStream = useCallback(() => {
    console.log('useLogStream: stopStream called');
    setSseUrl(null);
    setIsStreaming(false);
  }, []);

  const clearLogs = useCallback(() => {
    console.log('useLogStream: clearLogs called');
    logsRef.current = '';
    logLinesRef.current = [];
    currentStepRef.current = null;
    setLogs('');
    setLogLines([]);
    setCurrentStep(null);
    setLogStreamError(null);
  }, []);

  useEffect(() => {
    if (!isConnected && !isConnecting) {
      setIsStreaming(false);
    }
  }, [isConnected, isConnecting]);

  return {
    logs,
    logLines,
    isStreaming,
    isConnected,
    logStreamError,
    currentStep,
    startStream,
    stopStream,
    clearLogs,
  };
};