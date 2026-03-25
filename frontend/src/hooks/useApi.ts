import { useState, useEffect, useCallback } from 'react';
import { api } from '../utils/api';

interface UseApiResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useApi<T>(path: string): UseApiResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.get<T>(path);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [path]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}

export function useMutation<TInput, TOutput>(
  method: 'post' | 'put' | 'delete',
  path: string,
): {
  mutate: (body?: TInput) => Promise<TOutput>;
  loading: boolean;
  error: string | null;
} {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mutate = useCallback(
    async (body?: TInput): Promise<TOutput> => {
      setLoading(true);
      setError(null);
      try {
        const result = await api[method]<TOutput>(path, body);
        return result;
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Unknown error';
        setError(msg);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [method, path],
  );

  return { mutate, loading, error };
}
