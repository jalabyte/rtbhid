import { useState } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Terminal, Network, Shield, Globe, BookOpen } from "lucide-react";

export default function Docs() {
  const [copied, setCopied] = useState<string | null>(null);

  const handleCopy = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopied(label);
    setTimeout(() => setCopied(null), 2000);
  };

  const configs = {
    bird: {
      title: "BIRD 2 / 3",
      icon: <Globe className="h-5 w-5" />,
      description: "Drop the feed into the kernel as blackhole routes",
      config: `# BIRD 2/3 - receiving the Nullwell blackhole feed
# Placeholders: YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET,
#               YOUR_V4_PREFIX / YOUR_V6_PREFIX (your own space, e.g. 203.0.113.0/24)

# Your own space: never blackhole it, whatever the feed says.
define MY_V4 = [ YOUR_V4_PREFIX+ ];
define MY_V6 = [ YOUR_V6_PREFIX+ ];

filter nullwell_in4 {
    if net ~ MY_V4 then reject "own space";
    if net.len < 8 then reject "wider than /8";
    if !((65535, 666) ~ bgp_community) then reject "no BLACKHOLE community";
    bgp_community.add((65535, 65281));   # NO_EXPORT (RFC 7999 3.2)
    dest = RTD_BLACKHOLE;                 # drop, never forward to the agent
    accept;
}

filter nullwell_in6 {
    if net ~ MY_V6 then reject "own space";
    if net.len < 16 then reject "wider than /16";
    if !((65535, 666) ~ bgp_community) then reject "no BLACKHOLE community";
    bgp_community.add((65535, 65281));
    dest = RTD_BLACKHOLE;
    accept;
}

protocol bgp nullwell {
    description "Nullwell blackhole feed";
    local as YOUR_ASN;
    neighbor AGENT_IP as AGENT_ASN;
    password "SHARED_SECRET";            # TCP MD5; set the same on the portal's Peers page

    ipv4 {
        import filter nullwell_in4;
        export none;                     # never send routes back
        receive limit 300000 action disable;   # runaway feed: drop session, fail open
    };
    ipv6 {
        import filter nullwell_in6;
        export none;
        receive limit 300000 action disable;
    };
}

# Make sure your kernel protocols export these routes so the blackholes
# reach the FIB (e.g. "export all;" or a filter matching dest = RTD_BLACKHOLE).`,
    },
    junos: {
      title: "Juniper Junos",
      icon: <Network className="h-5 w-5" />,
      description: "Import policy with next-hop discard",
      config: `# Juniper Junos - receiving the Nullwell blackhole feed
# Placeholders: YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET, YOUR_V4_PREFIX, YOUR_V6_PREFIX

set policy-options prefix-list MY-V4 YOUR_V4_PREFIX
set policy-options prefix-list MY-V6 YOUR_V6_PREFIX
set policy-options community BLACKHOLE members 65535:666
set policy-options community NO-EXPORT members no-export

set policy-options policy-statement NULLWELL-IN term own-v4 from prefix-list-filter MY-V4 orlonger
set policy-options policy-statement NULLWELL-IN term own-v4 then reject
set policy-options policy-statement NULLWELL-IN term own-v6 from prefix-list-filter MY-V6 orlonger
set policy-options policy-statement NULLWELL-IN term own-v6 then reject
set policy-options policy-statement NULLWELL-IN term wide-v4 from route-filter 0.0.0.0/0 upto /7
set policy-options policy-statement NULLWELL-IN term wide-v4 then reject
set policy-options policy-statement NULLWELL-IN term wide-v6 from route-filter ::/0 upto /15
set policy-options policy-statement NULLWELL-IN term wide-v6 then reject
set policy-options policy-statement NULLWELL-IN term blackhole from community BLACKHOLE
set policy-options policy-statement NULLWELL-IN term blackhole then community add NO-EXPORT
set policy-options policy-statement NULLWELL-IN term blackhole then next-hop discard
set policy-options policy-statement NULLWELL-IN term blackhole then accept
set policy-options policy-statement NULLWELL-IN then reject

set policy-options policy-statement REJECT-ALL then reject

set protocols bgp group NULLWELL type external
set protocols bgp group NULLWELL peer-as AGENT_ASN
set protocols bgp group NULLWELL import NULLWELL-IN
set protocols bgp group NULLWELL export REJECT-ALL
set protocols bgp group NULLWELL family inet unicast prefix-limit maximum 300000 teardown
set protocols bgp group NULLWELL family inet6 unicast prefix-limit maximum 300000 teardown
set protocols bgp group NULLWELL neighbor AGENT_IP authentication-key "SHARED_SECRET"`,
    },
    cisco: {
      title: "Cisco IOS / IOS-XE",
      icon: <Terminal className="h-5 w-5" />,
      description: "Route-map to a Null0 next hop",
      config: `! Cisco IOS / IOS-XE - receiving the Nullwell blackhole feed
! Placeholders: YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET, YOUR_V4_PREFIX, YOUR_V6_PREFIX

! Discard next hops (192.0.2.1 = TEST-NET-1, 100::/64 = RFC 6666 discard-only)
ip route 192.0.2.1 255.255.255.255 Null0
ipv6 route 100::1/128 Null0

ip prefix-list MY-V4 seq 5 permit YOUR_V4_PREFIX le 32
ip prefix-list WIDE-V4 seq 5 permit 0.0.0.0/0 le 7
ipv6 prefix-list MY-V6 seq 5 permit YOUR_V6_PREFIX le 128
ipv6 prefix-list WIDE-V6 seq 5 permit ::/0 le 15
ip community-list standard BLACKHOLE permit 65535:666

route-map NULLWELL-IN-V4 deny 10
 match ip address prefix-list MY-V4
route-map NULLWELL-IN-V4 deny 20
 match ip address prefix-list WIDE-V4
route-map NULLWELL-IN-V4 permit 30
 match community BLACKHOLE
 set ip next-hop 192.0.2.1
 set community no-export additive
!
route-map NULLWELL-IN-V6 deny 10
 match ipv6 address prefix-list MY-V6
route-map NULLWELL-IN-V6 deny 20
 match ipv6 address prefix-list WIDE-V6
route-map NULLWELL-IN-V6 permit 30
 match community BLACKHOLE
 set ipv6 next-hop 100::1
 set community no-export additive
!
route-map DENY-ALL deny 10
!
router bgp YOUR_ASN
 neighbor AGENT_IP remote-as AGENT_ASN
 neighbor AGENT_IP description Nullwell-feed
 neighbor AGENT_IP password SHARED_SECRET
 ! no ttl-security: the Nullwell speaker does not implement GTSM
 !
 address-family ipv4
  neighbor AGENT_IP activate
  neighbor AGENT_IP route-map NULLWELL-IN-V4 in
  neighbor AGENT_IP route-map DENY-ALL out
  neighbor AGENT_IP maximum-prefix 300000
 exit-address-family
 !
 address-family ipv6
  neighbor AGENT_IP activate
  neighbor AGENT_IP route-map NULLWELL-IN-V6 in
  neighbor AGENT_IP route-map DENY-ALL out
  neighbor AGENT_IP maximum-prefix 300000
 exit-address-family`,
    },
    arista: {
      title: "Arista EOS",
      icon: <Shield className="h-5 w-5" />,
      description: "Route-map to a Null0 next hop",
      config: `! Arista EOS - receiving the Nullwell blackhole feed
! Placeholders: YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET, YOUR_V4_PREFIX, YOUR_V6_PREFIX

ip route 192.0.2.1/32 Null0
ipv6 route 100::1/128 Null0

ip prefix-list MY-V4 seq 10 permit YOUR_V4_PREFIX le 32
ip prefix-list WIDE-V4 seq 10 permit 0.0.0.0/0 le 7
ipv6 prefix-list MY-V6
   seq 10 permit YOUR_V6_PREFIX le 128
ipv6 prefix-list WIDE-V6
   seq 10 permit ::/0 le 15
ip community-list BLACKHOLE permit 65535:666

route-map NULLWELL-IN deny 10
   match ip address prefix-list MY-V4
route-map NULLWELL-IN deny 20
   match ip address prefix-list WIDE-V4
route-map NULLWELL-IN deny 30
   match ipv6 address prefix-list MY-V6
route-map NULLWELL-IN deny 40
   match ipv6 address prefix-list WIDE-V6
route-map NULLWELL-IN permit 50
   match community BLACKHOLE
   set ip next-hop 192.0.2.1
   set ipv6 next-hop 100::1
   set community no-export additive
!
route-map DENY-ALL deny 10
!
router bgp YOUR_ASN
   neighbor AGENT_IP remote-as AGENT_ASN
   neighbor AGENT_IP description Nullwell-feed
   neighbor AGENT_IP password 0 SHARED_SECRET
   neighbor AGENT_IP maximum-routes 300000
   neighbor AGENT_IP route-map NULLWELL-IN in
   neighbor AGENT_IP route-map DENY-ALL out
   !
   address-family ipv4
      neighbor AGENT_IP activate
   !
   address-family ipv6
      neighbor AGENT_IP activate`,
    },
    huawei: {
      title: "Huawei VRP",
      icon: <Globe className="h-5 w-5" />,
      description: "Route-policy to a NULL0 next hop",
      config: `# Huawei VRP - receiving the Nullwell blackhole feed
# Placeholders: YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET,
#   YOUR_V4_NET YOUR_V4_LEN / YOUR_V6_NET YOUR_V6_LEN (your own space)

ip route-static 192.0.2.1 255.255.255.255 NULL0
ipv6 route-static 100::1 128 NULL0

ip ip-prefix MY-V4 index 10 permit YOUR_V4_NET YOUR_V4_LEN greater-equal YOUR_V4_LEN less-equal 32
ip ip-prefix WIDE-V4 index 10 permit 0.0.0.0 0 less-equal 7
ip ipv6-prefix MY-V6 index 10 permit YOUR_V6_NET YOUR_V6_LEN greater-equal YOUR_V6_LEN less-equal 128
ip ipv6-prefix WIDE-V6 index 10 permit :: 0 less-equal 15
ip community-filter basic BLACKHOLE permit 65535:666

route-policy NULLWELL-IN-V4 deny node 10
 if-match ip-prefix MY-V4
route-policy NULLWELL-IN-V4 deny node 20
 if-match ip-prefix WIDE-V4
route-policy NULLWELL-IN-V4 permit node 30
 if-match community-filter BLACKHOLE
 apply ip-address next-hop 192.0.2.1
 apply community no-export additive
#
route-policy NULLWELL-IN-V6 deny node 10
 if-match ipv6 address prefix-list MY-V6
route-policy NULLWELL-IN-V6 deny node 20
 if-match ipv6 address prefix-list WIDE-V6
route-policy NULLWELL-IN-V6 permit node 30
 if-match community-filter BLACKHOLE
 apply ipv6 next-hop 100::1
 apply community no-export additive
#
route-policy DENY-ALL deny node 10
#
bgp YOUR_ASN
 peer AGENT_IP as-number AGENT_ASN
 peer AGENT_IP description Nullwell-feed
 peer AGENT_IP password cipher SHARED_SECRET
 #
 ipv4-family unicast
  peer AGENT_IP enable
  peer AGENT_IP route-policy NULLWELL-IN-V4 import
  peer AGENT_IP route-policy DENY-ALL export
  peer AGENT_IP route-limit 300000
 #
 ipv6-family unicast
  peer AGENT_IP enable
  peer AGENT_IP route-policy NULLWELL-IN-V6 import
  peer AGENT_IP route-policy DENY-ALL export
  peer AGENT_IP route-limit 300000
#
return`,
    },
    mikrotik: {
      title: "MikroTik RouterOS",
      icon: <Terminal className="h-5 w-5" />,
      description: "RouterOS v7 filter that installs blackhole routes",
      config: `# MikroTik RouterOS v7 - receiving the Nullwell blackhole feed
# Placeholders: YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET, YOUR_V4_PREFIX, YOUR_V6_PREFIX

/routing filter rule
add chain=nullwell-in rule="if (dst in YOUR_V4_PREFIX) { reject }"
add chain=nullwell-in rule="if (dst in YOUR_V6_PREFIX) { reject }"
add chain=nullwell-in rule="if (afi ipv4 && dst-len < 8) { reject }"
add chain=nullwell-in rule="if (afi ipv6 && dst-len < 16) { reject }"
add chain=nullwell-in rule="if (bgp-communities includes 65535:666) { set blackhole yes; append bgp-communities no-export; accept }"
add chain=nullwell-in rule="reject"

# No output filter or redistribution: nothing is announced back.
/routing bgp connection
add name=nullwell as=YOUR_ASN local.role=ebgp \\
    remote.address=AGENT_IP remote.as=AGENT_ASN \\
    tcp-md5-key=SHARED_SECRET \\
    address-families=ip,ipv6 \\
    input.filter=nullwell-in \\
    input.limit-process-routes-ipv4=300000 \\
    input.limit-process-routes-ipv6=300000`,
    },
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-lg font-semibold">Documentation</h1>
          <p className="text-muted-foreground">
            Router configuration for networks receiving the Nullwell blackhole feed
          </p>
        </div>
        <Badge variant="outline" className="text-sm px-3 py-1">
          <BookOpen className="h-4 w-4 mr-1" />
          Public Access
        </Badge>
      </div>

      <Alert>
        <AlertDescription>
          Examples for networks that peer with a Nullwell agent to receive its blackhole feed. Replace the
          placeholders (YOUR_ASN, AGENT_ASN, AGENT_IP, SHARED_SECRET, YOUR_V4_PREFIX…) with your values, and test
          on a lab router first: syntax differs between software versions.
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle>Six rules every receiver must follow</CardTitle>
          <CardDescription>The examples below implement all of them. Skip one and the feed can hurt you.</CardDescription>
        </CardHeader>
        <CardContent>
          <ol className="list-decimal space-y-2 pl-5 text-muted-foreground">
            <li>
              <strong className="text-foreground">Drop, never forward.</strong> Point accepted routes at a discard
              next hop (192.0.2.1 and 100::1 routed to Null0) or install them as blackholes. Otherwise traffic for
              every listed prefix is sent to the agent host.
            </li>
            <li>
              <strong className="text-foreground">Never your own space.</strong> Reject any route inside your own
              prefixes, so a bad list entry can never cut off your customers or your infrastructure.
            </li>
            <li>
              <strong className="text-foreground">Nothing too wide.</strong> Reject IPv4 shorter than /8 and IPv6
              shorter than /16. The portal already refuses to announce these; filtering on your side as well
              protects you if anything upstream goes wrong.
            </li>
            <li>
              <strong className="text-foreground">Don't pass it on.</strong> Add NO_EXPORT (RFC 7999 §3.2) and
              export nothing to the agent.
            </li>
            <li>
              <strong className="text-foreground">Cap the session.</strong> A maximum-prefix of about 300,000 tears
              the session down if the feed runs away. Teardown withdraws every blackhole, so it fails open.
            </li>
            <li>
              <strong className="text-foreground">Authenticate it.</strong> Use TCP MD5 with the password set on the
              portal's Peers page. Don't enable ttl-security (GTSM): the Nullwell speaker doesn't support it and
              the session won't come up.
            </li>
          </ol>
        </CardContent>
      </Card>

      <Tabs defaultValue="bird" className="w-full">
        <TabsList className="grid w-full grid-cols-6">
          {Object.entries(configs).map(([key, config]) => (
            <TabsTrigger
              key={key}
              value={key}
              className="flex items-center gap-2"
            >
              {config.icon}
              <span className="hidden sm:inline">{config.title}</span>
            </TabsTrigger>
          ))}
        </TabsList>

        {Object.entries(configs).map(([key, config]) => (
          <TabsContent key={key} value={key}>
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  {config.icon}
                  {config.title}
                </CardTitle>
                <CardDescription>{config.description}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="relative">
                  <pre className="bg-muted p-4 rounded-md overflow-x-auto text-sm font-mono">
                    <code>{config.config}</code>
                  </pre>
                  <button
                    onClick={() => handleCopy(config.config, key)}
                    className="absolute top-2 right-2 px-3 py-1 bg-primary text-primary-foreground text-xs rounded-md hover:bg-primary/90 transition-colors"
                  >
                    {copied === key ? "Copied!" : "Copy"}
                  </button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        ))}
      </Tabs>

      <Card>
        <CardHeader>
          <CardTitle>Receiving the feed</CardTitle>
          <CardDescription>
            What arrives, and how to use it safely
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <section>
            <h2 className="mb-2 text-base font-semibold">What you receive</h2>
            <p className="text-muted-foreground">
              One route per listed prefix, IPv4 and IPv6, each carrying the BLACKHOLE community{" "}
              <code className="font-mono">65535:666</code> (RFC 7999). The operator may add large communities as
              well; match on 65535:666. The feed changes as blocklists sync, so expect regular updates, not a
              static table.
            </p>
          </section>

          <section>
            <h2 className="mb-2 text-base font-semibold">Dropping traffic from listed sources too</h2>
            <p className="text-muted-foreground">
              Destination blackholing stops traffic <em>to</em> listed prefixes. To also drop traffic{" "}
              <em>from</em> them, enable loose-mode uRPF on your edge interfaces (source-based RTBH, RFC 5635):
              packets whose source resolves to a discard route are dropped.
            </p>
          </section>

          <section>
            <h2 className="mb-2 text-base font-semibold">Before going live</h2>
            <ol className="list-decimal space-y-1 pl-5 text-muted-foreground">
              <li>Bring the session up and confirm it receives routes but sends none.</li>
              <li>Check a listed prefix resolves to the discard next hop or a blackhole route.</li>
              <li>Check your own prefixes are not present, even if you add one to the feed as a test.</li>
              <li>Alert on session state and on the received-prefix count.</li>
            </ol>
          </section>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Contact & Support</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground">
            For technical support or questions about RTBH integration, please
            contact the network operations team or visit our documentation
            portal at{" "}
            <a
              href="https://jalabyte.com"
              target="_blank"
              rel="noopener"
              className="text-primary hover:underline"
            >
              jalabyte.com
            </a>
            .
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
