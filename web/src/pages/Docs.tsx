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
      description: "Configure BIRD to receive RTBH routes from upstream",
      config: `# BIRD 2/3 Configuration - Receiving RTBH Routes
# As an upstream provider accepting RTBH advertisements

# Define RTBH community (standard and large)
define RTBH_STD_COMMUNITY = (65535, 666);
define RTBH_LARGE_COMMUNITY = (65535, 666, 666);

# RTBH import filter - accept and blackhole routes
filter rtbh_import {
    if bgp_community ~ [(65535, 666)] ||
       bgp_large_community ~ [(65535, 666, 666)] then {
        # Blackhole traffic to this prefix
        dest = RTD_BLACKHOLE;
        accept;
    }
    reject;
}

# RTBH peer configuration (accepting from customer)
protocol bgp CUSTOMER_RTBH {
    description "Customer RTBH peer";
    local as YOUR_ASN;
    neighbor CUSTOMER_IP as CUSTOMER_ASN;

    ipv4 {
        import filter rtbh_import;
        export none;  # Don't export anything back
    };

    ipv6 {
        import filter rtbh_import;
        export none;
    };
}`,
    },
    junos: {
      title: "Juniper Junos",
      icon: <Network className="h-5 w-5" />,
      description: "Junos configuration for RTBH route reception",
      config: `# Juniper Junos - Accepting RTBH Routes from Customer
# Upstream provider perspective

# Configure routing-options for discard
set routing-options static route 192.0.2.0/24 discard
set routing-options static route 198.51.100.0/24 discard

# Create routing policy to accept RTBH
set policy-options policy-statement RTBH-IMPORT term 1 from protocol bgp
set policy-options policy-statement RTBH-IMPORT term 1 from community RTBH
set policy-options policy-statement RTBH-IMPORT term 1 then accept

set policy-options policy-statement RTBH-IMPORT term default then reject

# Define RTBH community
set policy-options community RTBH members 65535:666

# Configure BGP peer from customer
set protocols bgp group CUSTOMER-RTBH type external
set protocols bgp group CUSTOMER-RTBH peer-as CUSTOMER_ASN
set protocols bgp group CUSTOMER-RTBH neighbor CUSTOMER_IP
set protocols bgp group CUSTOMER-RTBH neighbor CUSTOMER_IP import RTBH-IMPORT
set protocols bgp group CUSTOMER-RTBH neighbor CUSTOMER_IP family inet unicast

# Optional: Configure flow routes for dynamic blackholing
set routing-options flow route 192.0.2.0/24 match destination 192.0.2.0/24
set routing-options flow route 192.0.2.0/24 then discard`,
    },
    cisco: {
      title: "Cisco IOS / IOS-XE",
      icon: <Terminal className="h-5 w-5" />,
      description: "Cisco configuration to receive and blackhole RTBH routes",
      config: `! Cisco IOS - Receiving RTBH Routes from Customer
! Upstream provider configuration

! Define RTBH community
ip community-list standard RTBH-COMMUNITY permit 65535:666

! Create route-map to accept RTBH routes
route-map RTBH-IMPORT permit 10
 match community RTBH-COMMUNITY
 set ip next-hop 192.0.2.1  ! Blackhole next-hop
!
route-map RTBH-IMPORT deny 20

! Configure static blackhole routes
ip route 192.0.2.0 255.255.255.0 null0
ip route 198.51.100.0 255.255.255.0 null0

! Configure BGP peer from customer
router bgp YOUR_ASN
 bgp log-neighbor-changes
 neighbor CUSTOMER_IP remote-as CUSTOMER_ASN
 neighbor CUSTOMER_IP description Customer-RTBH
 !
 address-family ipv4
  neighbor CUSTOMER_IP activate
  neighbor CUSTOMER_IP soft-reconfiguration inbound
  neighbor CUSTOMER_IP route-map RTBH-IMPORT in
 exit-address-family
 !
 address-family ipv6
  neighbor CUSTOMER_IP activate
  neighbor CUSTOMER_IP route-map RTBH-IMPORT in
 exit-address-family

! Alternative: Use BGP FlowSpec for dynamic filtering
! (Requires specific hardware/software support)`,
    },
    arista: {
      title: "Arista EOS",
      icon: <Shield className="h-5 w-5" />,
      description: "Arista EOS configuration for RTBH acceptance",
      config: `! Arista EOS - Accepting RTBH Routes
! Upstream provider perspective

! Define RTBH community
ip community-list standard RTBH-COMMUNITY permit 65535:666

! Create route-map for RTBH import
route-map RTBH-IMPORT permit 10
   match community RTBH-COMMUNITY
   set ip next-hop 192.0.2.1
!
route-map RTBH-IMPORT deny 20

! Configure static null routes
ip route 192.0.2.0/24 null0
ip route 198.51.100.0/24 null0

! Configure BGP
router bgp YOUR_ASN
   router-id YOUR_ROUTER_ID

   neighbor CUSTOMER-RTBH peer-group
   neighbor CUSTOMER-RTBH remote-as CUSTOMER_ASN
   neighbor CUSTOMER-RTBH description Customer-RTBH
   neighbor CUSTOMER-RTBH ebgp-multihop 2
   neighbor CUSTOMER-RTBH send-community

   neighbor CUSTOMER_IP peer-group CUSTOMER-RTBH

   address-family ipv4
      neighbor CUSTOMER-RTBH activate
      neighbor CUSTOMER-RTBH route-map RTBH-IMPORT in
      neighbor CUSTOMER-RTBH soft-reconfiguration inbound
   !
   address-family ipv6
      neighbor CUSTOMER-RTBH activate
      neighbor CUSTOMER-RTBH route-map RTBH-IMPORT in

! Optional: Use BGP FlowSpec
! router bgp YOUR_ASN
!    address-family ipv4 flowspec
!       neighbor CUSTOMER_IP activate`,
    },
    huawei: {
      title: "Huawei VRP",
      icon: <Globe className="h-5 w-5" />,
      description: "Huawei VRP configuration for RTBH route reception",
      config: `# Huawei VRP - Accepting RTBH Routes from Customer
# Upstream provider configuration

# Configure BGP community filter
ip community-filter 1 permit 65535:666

# Configure route-policy for RTBH
route-policy RTBH-IMPORT permit node 10
 if-match community-filter 1
 apply cost 666
#
route-policy RTBH-IMPORT deny node 20

# Configure static blackhole routes
ip route-static 192.0.2.0 255.255.255.0 NULL0
ip route-static 198.51.100.0 255.255.255.0 NULL0

# Configure BGP
bgp YOUR_ASN
 peer CUSTOMER_IP as-number CUSTOMER_ASN
 peer CUSTOMER_IP description Customer-RTBH
 peer CUSTOMER_IP ebgp-max-hop 2
 #
 ipv4-family unicast
  peer CUSTOMER_IP enable
  peer CUSTOMER_IP route-policy RTBH-IMPORT import
  peer CUSTOMER_IP advertise-community
 #
 ipv6-family unicast
  peer CUSTOMER_IP enable
  peer CUSTOMER_IP route-policy RTBH-IMPORT import
#
return`,
    },
    mikrotik: {
      title: "MikroTik RouterOS",
      icon: <Terminal className="h-5 w-5" />,
      description: "MikroTik configuration to receive RTBH routes",
      config: `# MikroTik RouterOS - Accepting RTBH Routes
# Upstream provider perspective

# Create routing filter to accept RTBH
/routing filter
add action=accept chain=RTBH-IN bgp-communities=65535:666 \
    comment="Accept RTBH routes"

# Configure BGP instance
/routing bgp instance
set default as=YOUR_ASN router-id=YOUR_ROUTER_ID

# Configure BGP peer from customer
/routing bgp peer
add name=CUSTOMER-RTBH remote-address=CUSTOMER_IP remote-as=CUSTOMER_ASN \
    in-filter=RTBH-IN \
    address-families=ip,ipv6

# Create blackhole routes (static null routes)
/ip route
add dst-address=192.0.2.0/24 type=blackhole comment="RTBH"
add dst-address=198.51.100.0/24 type=blackhole comment="RTBH"

# For dynamic blackholing, create routing rules
/routing rule
add action=drop dst-address=192.0.2.0/24 comment="RTBH Drop"
add action=drop dst-address=198.51.100.0/24 comment="RTBH Drop"

# Alternative: Use firewall to drop traffic
/ip firewall filter
add action=drop chain=forward dst-address=192.0.2.0/24 comment="RTBH"
add action=drop chain=forward dst-address=198.51.100.0/24 comment="RTBH"`,
    },
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">Documentation</h2>
          <p className="text-muted-foreground">
            Configuration guides for upstream providers receiving RTBH routes
          </p>
        </div>
        <Badge variant="outline" className="text-sm px-3 py-1">
          <BookOpen className="h-4 w-4 mr-1" />
          Public Access
        </Badge>
      </div>

      <Alert>
        <AlertDescription>
          This documentation provides configuration examples for{" "}
          <strong>upstream providers and peers</strong>
          who want to receive and act on RTBH (Remotely Triggered Black Hole)
          routes from this portal. Replace YOUR_ASN, CUSTOMER_ASN, CUSTOMER_IP,
          and other placeholders with actual values. Please be cautious some of
          this sample / example configuration may not work in your devices
          because of wrong syntax (some of the config is got from Internet) or
          typo or not supported.
        </AlertDescription>
      </Alert>

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
          <CardTitle>Understanding RTBH from Provider Perspective</CardTitle>
          <CardDescription>
            How upstream providers handle RTBH advertisements
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <section>
            <h3 className="font-semibold text-lg mb-2">What You Receive</h3>
            <p className="text-muted-foreground">
              As an upstream provider, you will receive BGP announcements from
              the RTBH portal with special BGP communities (65535:666 or large
              community 65535:666:666). These routes indicate which prefixes
              should be blackholed to mitigate DDoS attacks.
            </p>
          </section>

          <section>
            <h3 className="font-semibold text-lg mb-2">Required Actions</h3>
            <ol className="list-decimal list-inside space-y-2 text-muted-foreground">
              <li>
                <strong>Accept Routes</strong>: Configure BGP to accept routes
                with RTBH communities
              </li>
              <li>
                <strong>Blackhole Traffic</strong>: Set next-hop to
                null/discarded interface
              </li>
              <li>
                <strong>Don't Re-advertise</strong>: Do not export RTBH routes
                to other peers
              </li>
              <li>
                <strong>Monitor</strong>: Track which prefixes are currently
                blackholed
              </li>
            </ol>
          </section>

          <section>
            <h3 className="font-semibold text-lg mb-2">BGP Communities</h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="bg-muted p-3 rounded-md">
                <code className="text-sm font-mono">65535:666</code>
                <p className="text-xs text-muted-foreground mt-1">
                  Standard community for RTBH
                </p>
              </div>
              <div className="bg-muted p-3 rounded-md">
                <code className="text-sm font-mono">65535:666:666</code>
                <p className="text-xs text-muted-foreground mt-1">
                  Large community for RTBH
                </p>
              </div>
            </div>
          </section>

          <section>
            <h3 className="font-semibold text-lg mb-2">Integration Steps</h3>
            <ol className="list-decimal list-inside space-y-2 text-muted-foreground">
              <li>Configure BGP peering with the RTBH portal (this system)</li>
              <li>Set up import filters to recognize RTBH communities</li>
              <li>Configure blackhole/discard routes or null next-hops</li>
              <li>Test the setup with a non-production prefix</li>
              <li>Monitor BGP sessions and route updates</li>
            </ol>
          </section>

          <section>
            <h3 className="font-semibold text-lg mb-2">
              Security Considerations
            </h3>
            <ul className="list-disc list-inside space-y-1 text-muted-foreground">
              <li>
                Validate RTBH routes are only accepted from authorized peers
              </li>
              <li>Implement maximum prefix limits to prevent route overflow</li>
              <li>Use BGP authentication (MD5 or TCP-AO) where possible</li>
              <li>Monitor for unexpected RTBH advertisements</li>
              <li>Have a rollback plan for accidental blackholing</li>
            </ul>
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
