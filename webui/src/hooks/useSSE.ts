import { useEffect, useRef, useState, useCallback } from 'react';

export interface SSEMessage {
  event: string;
  data: string;
  id?: string;
}

export interface SSEHookOptions {
  onMessage?: (message: SSEMessage) => void;
  onError?: (error: Event) => void;
  onOpen?: () => void;
  onClose?: () => void;
  autoReconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

export interface SSEHookReturn {
  isConnected: boolean;
  isConnecting: boolean;
  error: string | null;
  connect: () => void;
  disconnect: () => void;
  lastMessage: SSEMessage | null;
}

export const useSSE = (url: string | null, options: SSEHookOptions = {}): SSEHookReturn => {
  const {
    onMessage,
    onError,
    onOpen,
    onClose,
    autoReconnect = true,
    reconnectInterval = 3000,
    maxReconnectAttempts = 5,
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastMessage, setLastMessage] = useState<SSEMessage | null>(null);

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const shouldReconnectRef = useRef(true);

  const cleanup = useCallback(() => {
    console.log('[useSSE] cleanup() - closing EventSource');
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  const connect = useCallback(async () => {
    console.log('[useSSE] connect() called, url:', url, 'existing EventSource:', !!eventSourceRef.current);
    if (!url) {
      console.log('[useSSE] connect() aborted - no url');
      return;
    }

    cleanup();
    setIsConnecting(true);
    setError(null);
    shouldReconnectRef.current = true;

    try {
      const finalUrl = url.startsWith('http')
      ? url
      : `${window.location.origin}${url.startsWith('/') ? url : `/${url}`}`;
      console.log('[useSSE] Connecting to EventSource:', finalUrl);

      const eventSource = new EventSource(finalUrl, { withCredentials: true });
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log('[useSSE] EventSource opened');
        setIsConnected(true);
        setIsConnecting(false);
        setError(null);
        reconnectAttemptsRef.current = 0;
        onOpen?.();
      };

      eventSource.onerror = (err) => {
        console.error('[useSSE] EventSource error:', err);
        setIsConnected(false);
        setIsConnecting(false);
        setError('Connection error');
        onError?.(err as Event);

        // Close and cleanup
        if (eventSourceRef.current === eventSource) {
          eventSource.close();
          eventSourceRef.current = null;
        }

        if (shouldReconnectRef.current && autoReconnect && reconnectAttemptsRef.current < maxReconnectAttempts) {
          reconnectAttemptsRef.current++;
          reconnectTimeoutRef.current = setTimeout(() => {
            if (shouldReconnectRef.current) {
              connect();
            }
          }, reconnectInterval);
        }
      };

      eventSource.onmessage = (event) => {
        const message: SSEMessage = {
          event: 'message',
          data: event.data,
          id: event.lastEventId || undefined,
        };
        setLastMessage(message);
        onMessage?.(message);
      };

      // Register handlers for all known event types
      const knownEvents = ['connected', 'log', 'step', 'waiting', 'ping', 'completed', 'error', 'disconnected'];
      knownEvents.forEach(eventType => {
        eventSource.addEventListener(eventType, (event: MessageEvent) => {
          const message: SSEMessage = {
            event: eventType,
            data: event.data,
            id: event.lastEventId || undefined,
          };
          setLastMessage(message);
          onMessage?.(message);
        });
      });

    } catch (err) {
      console.error('[useSSE] EventSource setup error:', err);
      setIsConnected(false);
      setIsConnecting(false);
      setError(err instanceof Error ? err.message : 'Unknown error');
    }
  }, [url, onMessage, onError, onOpen, autoReconnect, reconnectInterval, maxReconnectAttempts, cleanup]);

  const disconnect = useCallback(() => {
    console.log('[useSSE] disconnect() called');
    shouldReconnectRef.current = false;
    cleanup();
    setIsConnected(false);
    setIsConnecting(false);
    onClose?.();
  }, [cleanup, onClose]);

  // Auto-connect when URL changes and cleanup on unmount
  useEffect(() => {
    console.log('[useSSE] useEffect triggered, url:', url);
    if (url) {
      connect();
    } else {
      disconnect();
    }

    return () => {
      console.log('[useSSE] useEffect cleanup running');
      disconnect();
    };
  }, [url]); // eslint-disable-line react-hooks/exhaustive-deps

  return {
    isConnected,
    isConnecting,
    error,
    connect,
    disconnect,
    lastMessage,
  };
};

