import { getApiData, postApiData } from "./client";
import type { AuthUser, LoginResponse } from "../types";

export interface LoginRequest {
  username: string;
  password: string;
}

export async function login(request: LoginRequest): Promise<LoginResponse> {
  return await postApiData<LoginResponse, LoginRequest>("/api/v1/auth/login", request);
}

export async function fetchCurrentUser(): Promise<AuthUser> {
  return await getApiData<AuthUser>("/api/v1/auth/me");
}
