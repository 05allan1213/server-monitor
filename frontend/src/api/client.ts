import axios, { AxiosError, type AxiosRequestConfig } from "axios";

import type { ApiResponse } from "../types";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

const httpClient = axios.create({
  baseURL: apiBaseUrl,
  timeout: 10000,
});

export async function getApiData<T>(
  url: string,
  config: AxiosRequestConfig = {},
): Promise<T> {
  try {
    const response = await httpClient.get<ApiResponse<T>>(url, config);
    const payload = response.data;

    if (payload.status !== "success") {
      throw new Error(payload.error ?? "Unknown API error");
    }

    if (payload.data === undefined) {
      throw new Error("API response missing data field");
    }

    return payload.data;
  } catch (err) {
    if (axios.isAxiosError(err)) {
      throw normalizeAxiosError(err);
    }
    throw err;
  }
}

export async function getApiResponse<T>(
  url: string,
  config: AxiosRequestConfig = {},
): Promise<ApiResponse<T>> {
  try {
    const response = await httpClient.get<ApiResponse<T>>(url, {
      ...config,
      validateStatus: (status) => status < 600,
    });

    return response.data;
  } catch (err) {
    if (axios.isAxiosError(err)) {
      throw normalizeAxiosError(err);
    }
    throw err;
  }
}

function normalizeAxiosError(err: AxiosError<ApiResponse<unknown>>): Error {
  const payloadError = err.response?.data?.error;
  if (payloadError) {
    return new Error(payloadError);
  }

  if (err.response) {
    return new Error(`Request failed with status ${err.response.status}`);
  }

  if (err.code === AxiosError.ETIMEDOUT || err.code === "ECONNABORTED") {
    return new Error("Request timed out");
  }

  return new Error(err.message || "Request failed");
}
