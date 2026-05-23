import AsyncStorage from '@react-native-async-storage/async-storage';

export const TOKEN_KEY = 'koschei_token';

export const auth = {
  getToken: () => AsyncStorage.getItem(TOKEN_KEY),
  setToken: (token: string) => AsyncStorage.setItem(TOKEN_KEY, token),
  clearToken: () => AsyncStorage.removeItem(TOKEN_KEY),
};
