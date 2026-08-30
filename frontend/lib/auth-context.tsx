"use client";

import React, { createContext, useContext, useState } from "react";

const TOKEN_STORAGE_KEY = "docuwave_token";

function readStoredToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_STORAGE_KEY);
}

type AuthContextValue = {
  token: string | null;
  setToken: (token: string) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setTokenState] = useState<string | null>(readStoredToken);

  const setToken = (newToken: string) => {
    window.localStorage.setItem(TOKEN_STORAGE_KEY, newToken);
    setTokenState(newToken);
  };

  const logout = () => {
    window.localStorage.removeItem(TOKEN_STORAGE_KEY);
    setTokenState(null);
  };

  return (
    <AuthContext.Provider value={{ token, setToken, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
