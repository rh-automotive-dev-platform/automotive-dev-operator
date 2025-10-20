import { useEffect, useRef, useState, useCallback } from 'react';

export interface PollingMessage {
  event: string;
  data: string;
  id?: string;
}

export interface PollingHookOptions {
  onMessage?: (message: PollingMessage) => void;
  onError?: (error: Error) => void;
  onOpen?: () => void;
  onClose?: () => void;
  pollInterval?: number;
}

export interface PollingHookReturn {
  isConnected: boolean;
  isConnecting: boolean;
  error: string | null;
  connect: () => void;
  disconnect: () => void;
  lastMessage: PollingMessage | null;
}

export const usePolling = (url: string | null, options: PollingHookOptions = {}): PollingHookReturn => {
  const {
    onMessage,
    onError,
    onOpen,
    onClose,
    pollInterval = 2000, // Poll every 2 seconds
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastMessage, setLastMessage] = useState<PollingMessage | null>(null);

  const intervalRef = useRef<NodeJS.Timeout | null>(null);
  const shouldPollRef = useRef(false);
  const previousDataRef = useRef<any>(null);

  const cleanup = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const pollData = useCallback(async () => {
    if (!url || !shouldPollRef.current) return;

    try {
      const response = await fetch(url, {
        credentials: 'same-origin',
        headers: {
          'Accept': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const data = await response.json();

      if (!isConnected) {
        setIsConnected(true);
        setIsConnecting(false);
        setError(null);
        const connectedMsg: PollingMessage = {
          event: 'connected',
          data: 'Polling connected',
        };
        setLastMessage(connectedMsg);
        onMessage?.(connectedMsg);
        const initialMsg: PollingMessage = {
          event: 'initial-list',
          data: JSON.stringify(data),
        };
        setLastMessage(initialMsg);
        onMessage?.(initialMsg);

        previousDataRef.current = JSON.stringify(data);
        onOpen?.();
        return;
      }

      const currentDataStr = JSON.stringify(data);
      if (currentDataStr !== previousDataRef.current) {
        const updateMsg: PollingMessage = {
          event: 'initial-list',
          data: currentDataStr,
        };
        setLastMessage(updateMsg);
        onMessage?.(updateMsg);

        previousDataRef.current = currentDataStr;
      }

    } catch (err) {
      console.error('[usePolling] Fetch error:', err);
      setIsConnected(false);
      setError(err instanceof Error ? err.message : 'Unknown error');
      const errorObj = err instanceof Error ? err : new Error('Unknown error');
      onError?.(errorObj);
    }
  }, [url, onMessage, onError, onOpen, isConnected]);

  const connect = useCallback(() => {
    console.log('[usePolling] connect() called, url:', url);
    if (!url) {
      console.log('[usePolling] connect() aborted - no url');
      return;
    }

    cleanup();
    setIsConnecting(true);
    setError(null);
    shouldPollRef.current = true;
    previousDataRef.current = null;

    // Poll immediately
    pollData();

    // Set up polling interval
    intervalRef.current = setInterval(() => {
      pollData();
    }, pollInterval);
  }, [url, pollInterval, pollData, cleanup]);

  const disconnect = useCallback(() => {
    console.log('[usePolling] disconnect() called');
    shouldPollRef.current = false;
    cleanup();
    setIsConnected(false);
    setIsConnecting(false);
    previousDataRef.current = null;
    onClose?.();
  }, [cleanup, onClose]);

  // Auto-connect when URL changes
  useEffect(() => {
    console.log('[usePolling] useEffect triggered, url:', url);
    if (url) {
      connect();
    } else {
      disconnect();
    }

    return () => {
      console.log('[usePolling] useEffect cleanup running');
      disconnect();
    };
  }, [url]); // eslint-disable-line react-hooks/exhaustive-deps

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      disconnect();
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return {
    isConnected,
    isConnecting,
    error,
    connect,
    disconnect,
    lastMessage,
  };
};

