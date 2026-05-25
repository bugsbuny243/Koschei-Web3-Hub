import { useEffect, useMemo, useRef } from 'react';
import { Link } from 'expo-router';
import {
  Animated,
  Easing,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from 'react-native';

const BG = '#02030a';
const PANEL = 'rgba(8,16,31,0.78)';
const CYAN = '#00eaff';
const GREEN = '#00ff9d';
const VIOLET = '#9d4edd';
const MAGENTA = '#ff39d8';

type ModuleItem = {
  icon: string;
  title: string;
  description: string;
  status: 'ACTIVE' | 'READY';
  accent: string;
};

const MODULES: ModuleItem[] = [
  { icon: '⌬', title: 'Command Center', description: 'Operasyon ve kontrol merkezi', status: 'ACTIVE', accent: CYAN },
  { icon: '◈', title: 'Code Engine', description: 'Kod üretim atölyesi', status: 'ACTIVE', accent: GREEN },
  { icon: '▦', title: 'Image Forge', description: 'Görsel üretim laboratuvarı', status: 'READY', accent: MAGENTA },
  { icon: '▣', title: 'Video Lab', description: 'Sahne ve render odası', status: 'READY', accent: VIOLET },
  { icon: '◊', title: 'Audio Core', description: 'Ses ve konuşma stüdyosu', status: 'READY', accent: CYAN },
  { icon: '⬢', title: 'Chat Core', description: 'Canlı AI sohbet merkezi', status: 'ACTIVE', accent: GREEN },
  { icon: '⟁', title: 'Reason Matrix', description: 'Strateji ve karar odası', status: 'ACTIVE', accent: VIOLET },
];

const PIPELINE = ['INTAKE', 'BLUEPRINT', 'ARCHITECTURE', 'FILE PLAN', 'BUILD', 'REVIEW', 'DELIVERY'];

function Ring({ size, color, duration, reverse }: { size: number; color: string; duration: number; reverse?: boolean }) {
  const spin = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const loop = Animated.loop(
      Animated.timing(spin, { toValue: 1, duration, easing: Easing.linear, useNativeDriver: true }),
    );
    loop.start();
    return () => loop.stop();
  }, [duration, spin]);

  const rotate = spin.interpolate({
    inputRange: [0, 1],
    outputRange: reverse ? ['360deg', '0deg'] : ['0deg', '360deg'],
  });

  return (
    <Animated.View
      style={[
        styles.ring,
        {
          width: size,
          height: size,
          borderRadius: size / 2,
          borderTopColor: color,
          borderRightColor: 'rgba(255,255,255,0.08)',
          borderBottomColor: 'rgba(0,234,255,0.16)',
          borderLeftColor: 'rgba(157,78,221,0.2)',
          transform: [{ rotate }],
        },
      ]}
    />
  );
}

function MatrixRain({ left, delay, duration }: { left: number; delay: number; duration: number }) {
  const y = useRef(new Animated.Value(-250)).current;

  useEffect(() => {
    const loop = Animated.loop(
      Animated.timing(y, { toValue: 1050, duration, delay, easing: Easing.linear, useNativeDriver: true }),
    );
    loop.start();
    return () => loop.stop();
  }, [delay, duration, y]);

  const glyphs = '01KOSCHEIΨλΔ'.split('');

  return (
    <Animated.View style={[styles.rainCol, { left, transform: [{ translateY: y }] }]}>
      {Array.from({ length: 14 }).map((_, i) => (
        <Text key={i} style={[styles.rainText, { opacity: 1 - i / 14 }]}>
          {glyphs[(i + left) % glyphs.length]}
        </Text>
      ))}
    </Animated.View>
  );
}

function QuantumCore() {
  const pulse = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(pulse, { toValue: 1, duration: 1200, useNativeDriver: true }),
        Animated.timing(pulse, { toValue: 0, duration: 1200, useNativeDriver: true }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [pulse]);

  const scale = pulse.interpolate({ inputRange: [0, 1], outputRange: [1, 1.08] });

  return (
    <View style={styles.coreWrap}>
      <Ring size={260} color={CYAN} duration={8400} />
      <Ring size={210} color={MAGENTA} duration={5600} reverse />
      <Ring size={162} color={VIOLET} duration={4000} />
      <Ring size={118} color={GREEN} duration={2800} reverse />

      <Animated.View style={[styles.core, { transform: [{ scale }] }]}>
        <Text style={styles.coreTitle}>KOSCHEI</Text>
        <Text style={styles.coreSub}>THE IMMORTAL AI</Text>
      </Animated.View>

      <View style={[styles.statChip, { top: 6, left: 0 }]}>
        <Text style={styles.statNum}>12</Text><Text style={styles.statLbl}>AI MODELS</Text>
      </View>
      <View style={[styles.statChip, { top: 24, right: -4 }]}>
        <Text style={styles.statNum}>7</Text><Text style={styles.statLbl}>MODULES</Text>
      </View>
      <View style={[styles.statChip, { bottom: 18, left: 12 }]}>
        <Text style={styles.statNum}>LIVE</Text><Text style={styles.statLbl}>RUNTIME</Text>
      </View>
      <View style={[styles.statChip, { bottom: 2, right: 6 }]}>
        <Text style={styles.statNum}>READY</Text><Text style={styles.statLbl}>CREDITS</Text>
      </View>
    </View>
  );
}

function ModuleCard({ item }: { item: ModuleItem }) {
  const glow = useRef(new Animated.Value(0)).current;

  const onIn = () => Animated.timing(glow, { toValue: 1, duration: 180, useNativeDriver: false }).start();
  const onOut = () => Animated.timing(glow, { toValue: 0, duration: 220, useNativeDriver: false }).start();

  const borderColor = glow.interpolate({ inputRange: [0, 1], outputRange: ['rgba(255,255,255,0.1)', item.accent] });

  return (
    <Pressable onPressIn={onIn} onPressOut={onOut}>
      <Animated.View style={[styles.moduleCard, { borderColor }]}>
        <View style={styles.moduleHead}>
          <Text style={[styles.moduleIcon, { color: item.accent }]}>{item.icon}</Text>
          <Text style={[styles.statusPill, { color: item.status === 'ACTIVE' ? GREEN : CYAN }]}>{item.status}</Text>
        </View>
        <Text style={styles.moduleTitle}>{item.title}</Text>
        <Text style={styles.moduleDesc}>{item.description}</Text>
      </Animated.View>
    </Pressable>
  );
}

export default function UiLab() {
  const { width } = useWindowDimensions();
  const rainCols = useMemo(() => {
    const count = Math.min(14, Math.max(8, Math.floor(width / 42)));
    return Array.from({ length: count }).map((_, i) => ({ left: i * 44 + 2, delay: i * 200, duration: 5000 + (i % 4) * 900 }));
  }, [width]);

  return (
    <View style={styles.root}>
      <View style={styles.grid} pointerEvents="none" />
      {rainCols.map((r, i) => <MatrixRain key={i} left={r.left} delay={r.delay} duration={r.duration} />)}

      <ScrollView contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
        <View style={styles.topBar}>
          <Text style={styles.brand}>KOSCHEI</Text>
          <Text style={styles.online}>ONLINE</Text>
        </View>
        <Text style={styles.subtitle}>Quantum AI Command Universe</Text>

        <QuantumCore />

        <Text style={styles.section}>MODULE UNIVERSE</Text>
        <View style={styles.modulesWrap}>
          {MODULES.map((m) => <ModuleCard key={m.title} item={m} />)}
        </View>

        <View style={styles.runtimeCard}>
          <Text style={styles.runtimeTitle}>Runtime Factory</Text>
          <Text style={styles.runtimeDesc}>Proje üretim hangarı</Text>
          <View style={styles.pipelineWrap}>
            {PIPELINE.map((step, i) => (
              <View key={step} style={styles.pipelineRow}>
                <View style={styles.pipelineDot} />
                <Text style={styles.pipelineText}>{step}</Text>
                {i !== PIPELINE.length - 1 ? <Text style={styles.pipelineArrow}>→</Text> : null}
              </View>
            ))}
          </View>
        </View>

        <Link href="/" asChild>
          <Pressable style={styles.backBtn}><Text style={styles.backText}>Back to Gateway</Text></Pressable>
        </Link>

        <Text style={styles.footer}>KOSCHEI COMMAND UNIVERSE v0.1{"\n"}TRADEPIGLOBALL.CO</Text>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: BG },
  content: { paddingTop: 56, paddingHorizontal: 16, paddingBottom: 48 },
  topBar: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  brand: { color: '#f6fbff', fontSize: 26, fontWeight: '800', letterSpacing: 2 },
  online: { color: GREEN, borderWidth: 1, borderColor: 'rgba(0,255,157,0.5)', paddingHorizontal: 12, paddingVertical: 6, borderRadius: 99, fontSize: 12, fontWeight: '700' },
  subtitle: { color: 'rgba(198,225,255,0.85)', marginTop: 10, marginBottom: 22 },
  coreWrap: { alignSelf: 'center', width: 290, height: 290, justifyContent: 'center', alignItems: 'center', marginBottom: 22 },
  ring: { position: 'absolute', borderWidth: 1.6 },
  core: { width: 140, height: 140, borderRadius: 70, backgroundColor: 'rgba(11,32,53,0.95)', borderWidth: 1, borderColor: 'rgba(0,234,255,0.45)', justifyContent: 'center', alignItems: 'center', shadowColor: CYAN, shadowOpacity: 0.35, shadowRadius: 20 },
  coreTitle: { color: '#f3fbff', fontWeight: '800', letterSpacing: 1.3 },
  coreSub: { color: 'rgba(175,228,255,0.9)', fontSize: 10, marginTop: 6 },
  statChip: { position: 'absolute', backgroundColor: 'rgba(8,20,35,0.92)', borderColor: 'rgba(130,198,255,0.3)', borderWidth: 1, paddingHorizontal: 10, paddingVertical: 7, borderRadius: 10 },
  statNum: { color: '#fff', fontWeight: '700', fontSize: 11 },
  statLbl: { color: 'rgba(155,222,255,0.9)', fontSize: 9, marginTop: 2 },
  section: { color: '#d5eeff', fontSize: 12, letterSpacing: 2.2, marginBottom: 12, marginTop: 4 },
  modulesWrap: { gap: 10 },
  moduleCard: { backgroundColor: PANEL, borderWidth: 1, borderRadius: 16, padding: 14 },
  moduleHead: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  moduleIcon: { fontSize: 20, fontWeight: '700' },
  statusPill: { fontSize: 10, fontWeight: '800', letterSpacing: 0.8 },
  moduleTitle: { color: '#f1f9ff', fontSize: 17, fontWeight: '700', marginTop: 8 },
  moduleDesc: { color: 'rgba(178,210,238,0.9)', marginTop: 4, fontSize: 13 },
  runtimeCard: { marginTop: 16, backgroundColor: 'rgba(10,18,36,0.95)', borderRadius: 18, borderWidth: 1, borderColor: 'rgba(0,234,255,0.28)', padding: 16 },
  runtimeTitle: { color: '#f4fbff', fontSize: 24, fontWeight: '800' },
  runtimeDesc: { color: 'rgba(180,218,255,0.92)', marginTop: 6, marginBottom: 14 },
  pipelineWrap: { gap: 8 },
  pipelineRow: { flexDirection: 'row', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  pipelineDot: { width: 8, height: 8, borderRadius: 8, backgroundColor: CYAN },
  pipelineText: { color: '#dff6ff', fontSize: 12, fontWeight: '700', letterSpacing: 0.5 },
  pipelineArrow: { color: 'rgba(158,217,255,0.8)', fontWeight: '800' },
  backBtn: { marginTop: 16, borderWidth: 1, borderColor: 'rgba(0,234,255,0.5)', borderRadius: 999, alignSelf: 'flex-start', paddingHorizontal: 14, paddingVertical: 9 },
  backText: { color: CYAN, fontWeight: '700' },
  footer: { color: 'rgba(171,205,234,0.8)', textAlign: 'center', marginTop: 28, letterSpacing: 0.7, fontSize: 11 },
  grid: { ...StyleSheet.absoluteFillObject, opacity: 0.12, backgroundColor: 'transparent', borderTopWidth: 1, borderTopColor: 'rgba(0,229,255,0.2)' },
  rainCol: { position: 'absolute', top: 0 },
  rainText: { color: GREEN, fontSize: 10, lineHeight: 14, fontWeight: '700' },
});
