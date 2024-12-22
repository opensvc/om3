# Stage chez OpenSVC, partie om3

> **_NOTE:_** Ce dépot a été "fork" via le projet principal [om3](https://github.com/opensvc/om3) dans le cadre de mon stage de BUT 2 Informatique. 


OpenSVC est une entreprise qui développe, commercialise et supporte un logiciel open source. Ce logiciel est chargé de démarrer, arrêter et relocaliser les applications du client, et d’en assurer la réplication des données, afin d’en assurer la haute disponibilité. 
C’est-à-dire que l’agent de OpenSVC va être capable de basculer des services hébergés sur un serveur vers un autre si ce premier a un incident technique. 

L’agent om3 va donc surveiller tous les évènements qui se produisent dans un cluster, un groupe de serveurs, pour assurer la haute disponibilité de leurs services.

C'est donc un logiciel utilisé par les administrateurs réseau devant garantir la fiabilité des clusters.

om3
===
Le projet principal est om3, la troisième version majeure de l’agent OpenSVC, développée en Golang. Il se compose d’un daemon, d’un Cluster Resource Manager et d’un client d’administration à distance.

La compilation du projet en Go produit deux binaires :

_**om**_ : 
Daemon Linux qui va gérer les requêtes et administrer le nœud en local.

_**ox**_ : 
Client d’administration pouvant envoyer des requêtes à distance vers des nœuds contenant un om.


🎯Mes missions
============

Ma première mission consistait à développer un gestionnaire (handler) d’API de type REST et les sous-commandes d’administration correspondantes dans les commandes ox et om.

Ce gestionnaire a pour but de gérer les informations de nœud ("serveur") telles que la version de système d’exploitation, la mémoire ou l'espace de stockage restant.

Exemples de requêtes appelant le gestionnaire d'API, que j'ai pu créer :

- GET /node/name{nodename}/system/asset
- GET /node/name/{nodename}/system/disks
- GET /node/name/{nodename}/system/packages
- GET /node/name/{nodename}/system/patches

Exemple de gestionnaire d'API :
[GET /node/name/{nodename}/system/disk](https://github.com/Alexandre-Meunier/om3/blob/31e6cbe079a2fd9f9f0d59e2ba1d575bb745c012/daemon/api/api.yaml#L1736)

Cette mission a été particulièrement laborieuse, car chaque type de donnée devait être transformé sous forme de structure en Go.

De plus, j’ai appris à créer des commandes Linux avec la librairie Cobra, très populaire dans l’écosystème Golang. Elle permet de créer des sous-commandes et de décrire les options supportées par chacune.

Exemples de commandes que j'ai pu créer :

- om node system disks --node node1
- om node system packages
- ox node system hardware
- ox node system user --node node2

Du côté du binaire om, je devais simplement chercher les données sur le nœud local. Cependant, pour le binaire ox, il a fallu envoyer des requêtes au nœud distant pour que le gestionnaire d'API puisse retourner les données du nœud distant.

Exemple d'un fichier qui s'occupe de transtyper des données du disque de stockage de type local en type d'API pour pouvoir le retourner en réponse d'une requête :
[https://github.com/Alexandre-Meunier/om3/blob/dev/daemon/daemonapi/get_node_system_disk.go](https://github.com/Alexandre-Meunier/om3/blob/dev/daemon/daemonapi/get_node_system_disk.go)

Cette mission m'a permis de m’initier dans le développement du logiciel et pour apprendre à utiliser les outils pour travailler.

J'ai pu réaliser d'autres missions tel que l'optimisation d'un gestionnaire d'API en utilisant des goroutines, permettant la parallélisation de plusieurs programmes en simultané.
J'ai aussi pu contribuer à l'amélioration de la gestion du cycle de vie du daemon.

> **_PS:_** OpenSVC développe également un projet nommé “collecteur”, qui permet de collecter les états et piloter l’ensemble des clusters d’un client.
Voir le projet [oc3](https://github.com/Alexandre-Meunier/oc3) qui est associé à om3 et auquel j'ai pu aussi contribuer.

💻Technologies et pratiques utilisées
======================

- Golang
- API REST
- Daemon Linux
- Librairie Cobra
- Intégration continue (CI)
- IDE Goland (Jetbrains)


📈Statistiques
============

- 21 Commits <span style="color: green;">7 443 ++</span> <span style="color: red;">2 321 --</span>
- 8 Pull Requests

